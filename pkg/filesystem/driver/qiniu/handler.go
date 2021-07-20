package qiniu

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"strings"
	"time"

	model "github.com/cloudreve/Cloudreve/v3/models"
	"github.com/cloudreve/Cloudreve/v3/pkg/filesystem/fsctx"
	"github.com/cloudreve/Cloudreve/v3/pkg/filesystem/response"
	"github.com/cloudreve/Cloudreve/v3/pkg/request"
	"github.com/cloudreve/Cloudreve/v3/pkg/serializer"
	"github.com/qiniu/api.v7/v7/auth/qbox"
	"github.com/qiniu/api.v7/v7/storage"
)

// Driver 本機策略適配器
type Driver struct {
	Policy *model.Policy
}

// List 列出給定路徑下的文件
func (handler Driver) List(ctx context.Context, base string, recursive bool) ([]response.Object, error) {
	base = strings.TrimPrefix(base, "/")
	if base != "" {
		base += "/"
	}

	var (
		delimiter string
		marker    string
		objects   []storage.ListItem
		commons   []string
	)
	if !recursive {
		delimiter = "/"
	}

	mac := qbox.NewMac(handler.Policy.AccessKey, handler.Policy.SecretKey)
	cfg := storage.Config{
		UseHTTPS: true,
	}
	bucketManager := storage.NewBucketManager(mac, &cfg)

	for {
		entries, folders, nextMarker, hashNext, err := bucketManager.ListFiles(
			handler.Policy.BucketName,
			base, delimiter, marker, 1000)
		if err != nil {
			return nil, err
		}
		objects = append(objects, entries...)
		commons = append(commons, folders...)
		if !hashNext {
			break
		}
		marker = nextMarker
	}

	// 處理列取結果
	res := make([]response.Object, 0, len(objects)+len(commons))
	// 處理目錄
	for _, object := range commons {
		rel, err := filepath.Rel(base, object)
		if err != nil {
			continue
		}
		res = append(res, response.Object{
			Name:         path.Base(object),
			RelativePath: filepath.ToSlash(rel),
			Size:         0,
			IsDir:        true,
			LastModify:   time.Now(),
		})
	}
	// 處理文件
	for _, object := range objects {
		rel, err := filepath.Rel(base, object.Key)
		if err != nil {
			continue
		}
		res = append(res, response.Object{
			Name:         path.Base(object.Key),
			Source:       object.Key,
			RelativePath: filepath.ToSlash(rel),
			Size:         uint64(object.Fsize),
			IsDir:        false,
			LastModify:   time.Unix(object.PutTime/10000000, 0),
		})
	}

	return res, nil
}

// Get 獲取文件
func (handler Driver) Get(ctx context.Context, path string) (response.RSCloser, error) {
	// 給檔案名加上隨機參數以強制拉取
	path = fmt.Sprintf("%s?v=%d", path, time.Now().UnixNano())

	// 獲取文件源地址
	downloadURL, err := handler.Source(
		ctx,
		path,
		url.URL{},
		int64(model.GetIntSetting("preview_timeout", 60)),
		false,
		0,
	)
	if err != nil {
		return nil, err
	}

	// 獲取文件資料流
	client := request.HTTPClient{}
	resp, err := client.Request(
		"GET",
		downloadURL,
		nil,
		request.WithContext(ctx),
		request.WithHeader(
			http.Header{"Cache-Control": {"no-cache", "no-store", "must-revalidate"}},
		),
		request.WithTimeout(time.Duration(0)),
	).CheckHTTPResponse(200).GetRSCloser()
	if err != nil {
		return nil, err
	}

	resp.SetFirstFakeChunk()

	// 嘗試自主獲取檔案大小
	if file, ok := ctx.Value(fsctx.FileModelCtx).(model.File); ok {
		resp.SetContentLength(int64(file.Size))
	}

	return resp, nil
}

// Put 將文件流儲存到指定目錄
func (handler Driver) Put(ctx context.Context, file io.ReadCloser, dst string, size uint64) error {
	defer file.Close()

	// 憑證有效期
	credentialTTL := model.GetIntSetting("upload_credential_timeout", 3600)

	// 生成上傳策略
	putPolicy := storage.PutPolicy{
		// 指定為覆蓋策略
		Scope:        fmt.Sprintf("%s:%s", handler.Policy.BucketName, dst),
		SaveKey:      dst,
		ForceSaveKey: true,
		FsizeLimit:   int64(size),
	}
	// 是否開啟了MIMEType限制
	if handler.Policy.OptionsSerialized.MimeType != "" {
		putPolicy.MimeLimit = handler.Policy.OptionsSerialized.MimeType
	}

	// 生成上傳憑證
	token, err := handler.getUploadCredential(ctx, putPolicy, int64(credentialTTL))
	if err != nil {
		return err
	}

	// 建立上傳表單
	cfg := storage.Config{}
	formUploader := storage.NewFormUploader(&cfg)
	ret := storage.PutRet{}
	putExtra := storage.PutExtra{
		Params: map[string]string{},
	}

	// 開始上傳
	err = formUploader.Put(ctx, &ret, token.Token, dst, file, int64(size), &putExtra)
	if err != nil {
		return err
	}

	return nil
}

// Delete 刪除一個或多個文件，
// 返回未刪除的文件
func (handler Driver) Delete(ctx context.Context, files []string) ([]string, error) {
	// TODO 大於一千個文件需要分批發送
	deleteOps := make([]string, 0, len(files))
	for _, key := range files {
		deleteOps = append(deleteOps, storage.URIDelete(handler.Policy.BucketName, key))
	}

	mac := qbox.NewMac(handler.Policy.AccessKey, handler.Policy.SecretKey)
	cfg := storage.Config{
		UseHTTPS: true,
	}
	bucketManager := storage.NewBucketManager(mac, &cfg)
	rets, err := bucketManager.Batch(deleteOps)

	// 處理刪除結果
	if err != nil {
		failed := make([]string, 0, len(rets))
		for k, ret := range rets {
			if ret.Code != 200 && ret.Code != 612 {
				failed = append(failed, files[k])
			}
		}
		return failed, errors.New("刪除失敗")
	}

	return []string{}, nil
}

// Thumb 獲取文件縮圖
func (handler Driver) Thumb(ctx context.Context, path string) (*response.ContentResponse, error) {
	var (
		thumbSize = [2]uint{400, 300}
		ok        = false
	)
	if thumbSize, ok = ctx.Value(fsctx.ThumbSizeCtx).([2]uint); !ok {
		return nil, errors.New("無法獲取縮圖尺寸設定")
	}

	path = fmt.Sprintf("%s?imageView2/1/w/%d/h/%d", path, thumbSize[0], thumbSize[1])
	return &response.ContentResponse{
		Redirect: true,
		URL: handler.signSourceURL(
			ctx,
			path,
			int64(model.GetIntSetting("preview_timeout", 60)),
		),
	}, nil
}

// Source 獲取外鏈URL
func (handler Driver) Source(
	ctx context.Context,
	path string,
	baseURL url.URL,
	ttl int64,
	isDownload bool,
	speed int,
) (string, error) {
	// 嘗試從上下文獲取檔案名
	fileName := ""
	if file, ok := ctx.Value(fsctx.FileModelCtx).(model.File); ok {
		fileName = file.Name
	}

	// 加入下載相關設定
	if isDownload {
		path = path + "?attname=" + url.PathEscape(fileName)
	}

	// 取得原始文件地址
	return handler.signSourceURL(ctx, path, ttl), nil
}

func (handler Driver) signSourceURL(ctx context.Context, path string, ttl int64) string {
	var sourceURL string
	if handler.Policy.IsPrivate {
		mac := qbox.NewMac(handler.Policy.AccessKey, handler.Policy.SecretKey)
		deadline := time.Now().Add(time.Second * time.Duration(ttl)).Unix()
		sourceURL = storage.MakePrivateURL(mac, handler.Policy.BaseURL, path, deadline)
	} else {
		sourceURL = storage.MakePublicURL(handler.Policy.BaseURL, path)
	}
	return sourceURL
}

// Token 獲取上傳策略和認證Token
func (handler Driver) Token(ctx context.Context, TTL int64, key string) (serializer.UploadCredential, error) {
	// 生成回調地址
	siteURL := model.GetSiteURL()
	apiBaseURI, _ := url.Parse("/api/v3/callback/qiniu/" + key)
	apiURL := siteURL.ResolveReference(apiBaseURI)

	// 讀取上下文中生成的儲存路徑
	savePath, ok := ctx.Value(fsctx.SavePathCtx).(string)
	if !ok {
		return serializer.UploadCredential{}, errors.New("無法獲取儲存路徑")
	}

	// 建立上傳策略
	putPolicy := storage.PutPolicy{
		Scope:            handler.Policy.BucketName,
		CallbackURL:      apiURL.String(),
		CallbackBody:     `{"name":"$(fname)","source_name":"$(key)","size":$(fsize),"pic_info":"$(imageInfo.width),$(imageInfo.height)"}`,
		CallbackBodyType: "application/json",
		SaveKey:          savePath,
		ForceSaveKey:     true,
		FsizeLimit:       int64(handler.Policy.MaxSize),
	}
	// 是否開啟了MIMEType限制
	if handler.Policy.OptionsSerialized.MimeType != "" {
		putPolicy.MimeLimit = handler.Policy.OptionsSerialized.MimeType
	}

	return handler.getUploadCredential(ctx, putPolicy, TTL)
}

// getUploadCredential 簽名上傳策略
func (handler Driver) getUploadCredential(ctx context.Context, policy storage.PutPolicy, TTL int64) (serializer.UploadCredential, error) {
	policy.Expires = uint64(TTL)
	mac := qbox.NewMac(handler.Policy.AccessKey, handler.Policy.SecretKey)
	upToken := policy.UploadToken(mac)

	return serializer.UploadCredential{
		Token: upToken,
	}, nil
}
