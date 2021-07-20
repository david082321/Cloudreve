package onedrive

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"path"
	"path/filepath"
	"strings"
	"time"

	model "github.com/cloudreve/Cloudreve/v3/models"
	"github.com/cloudreve/Cloudreve/v3/pkg/auth"
	"github.com/cloudreve/Cloudreve/v3/pkg/cache"
	"github.com/cloudreve/Cloudreve/v3/pkg/filesystem/fsctx"
	"github.com/cloudreve/Cloudreve/v3/pkg/filesystem/response"
	"github.com/cloudreve/Cloudreve/v3/pkg/request"
	"github.com/cloudreve/Cloudreve/v3/pkg/serializer"
)

// Driver OneDrive 適配器
type Driver struct {
	Policy     *model.Policy
	Client     *Client
	HTTPClient request.Client
}

// List 列取項目
func (handler Driver) List(ctx context.Context, base string, recursive bool) ([]response.Object, error) {
	base = strings.TrimPrefix(base, "/")
	// 列取子項目
	objects, _ := handler.Client.ListChildren(ctx, base)

	// 獲取真實的列取起始根目錄
	rootPath := base
	if realBase, ok := ctx.Value(fsctx.PathCtx).(string); ok {
		rootPath = realBase
	} else {
		ctx = context.WithValue(ctx, fsctx.PathCtx, base)
	}

	// 整理結果
	res := make([]response.Object, 0, len(objects))
	for _, object := range objects {
		source := path.Join(base, object.Name)
		rel, err := filepath.Rel(rootPath, source)
		if err != nil {
			continue
		}
		res = append(res, response.Object{
			Name:         object.Name,
			RelativePath: filepath.ToSlash(rel),
			Source:       source,
			Size:         object.Size,
			IsDir:        object.Folder != nil,
			LastModify:   time.Now(),
		})
	}

	// 遞迴列取子目錄
	if recursive {
		for _, object := range objects {
			if object.Folder != nil {
				sub, _ := handler.List(ctx, path.Join(base, object.Name), recursive)
				res = append(res, sub...)
			}
		}
	}

	return res, nil
}

// Get 獲取文件
func (handler Driver) Get(ctx context.Context, path string) (response.RSCloser, error) {
	// 獲取文件源地址
	downloadURL, err := handler.Source(
		ctx,
		path,
		url.URL{},
		60,
		false,
		0,
	)
	if err != nil {
		return nil, err
	}

	// 獲取文件資料流
	resp, err := handler.HTTPClient.Request(
		"GET",
		downloadURL,
		nil,
		request.WithContext(ctx),
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
	return handler.Client.Upload(ctx, dst, int(size), file)
}

// Delete 刪除一個或多個文件，
// 返回未刪除的文件，及遇到的最後一個錯誤
func (handler Driver) Delete(ctx context.Context, files []string) ([]string, error) {
	return handler.Client.BatchDelete(ctx, files)
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

	res, err := handler.Client.GetThumbURL(ctx, path, thumbSize[0], thumbSize[1])
	if err != nil {
		// 如果出現異常，就清空文件的pic_info
		if file, ok := ctx.Value(fsctx.FileModelCtx).(model.File); ok {
			file.UpdatePicInfo("")
		}
	}
	return &response.ContentResponse{
		Redirect: true,
		URL:      res,
	}, err
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
	cacheKey := fmt.Sprintf("onedrive_source_%d_%s", handler.Policy.ID, path)
	if file, ok := ctx.Value(fsctx.FileModelCtx).(model.File); ok {
		cacheKey = fmt.Sprintf("onedrive_source_file_%d_%d", file.UpdatedAt.Unix(), file.ID)
		// 如果是永久連結，則返回簽名後的中轉外鏈
		if ttl == 0 {
			signedURI, err := auth.SignURI(
				auth.General,
				fmt.Sprintf("/api/v3/file/source/%d/%s", file.ID, file.Name),
				ttl,
			)
			if err != nil {
				return "", err
			}
			return baseURL.ResolveReference(signedURI).String(), nil
		}

	}

	// 嘗試從快取中尋找
	if cachedURL, ok := cache.Get(cacheKey); ok {
		return handler.replaceSourceHost(cachedURL.(string))
	}

	// 快取不存在，重新獲取
	res, err := handler.Client.Meta(ctx, "", path)
	if err == nil {
		// 寫入新的快取
		cache.Set(
			cacheKey,
			res.DownloadURL,
			model.GetIntSetting("onedrive_source_timeout", 1800),
		)
		return handler.replaceSourceHost(res.DownloadURL)
	}
	return "", err
}

func (handler Driver) replaceSourceHost(origin string) (string, error) {
	if handler.Policy.OptionsSerialized.OdProxy != "" {
		source, err := url.Parse(origin)
		if err != nil {
			return "", err
		}

		cdn, err := url.Parse(handler.Policy.OptionsSerialized.OdProxy)
		if err != nil {
			return "", err
		}

		// 取代反代地址
		source.Scheme = cdn.Scheme
		source.Host = cdn.Host
		return source.String(), nil
	}

	return origin, nil
}

// Token 獲取上傳工作階段URL
func (handler Driver) Token(ctx context.Context, TTL int64, key string) (serializer.UploadCredential, error) {

	// 讀取上下文中生成的儲存路徑和檔案大小
	savePath, ok := ctx.Value(fsctx.SavePathCtx).(string)
	if !ok {
		return serializer.UploadCredential{}, errors.New("無法獲取儲存路徑")
	}
	fileSize, ok := ctx.Value(fsctx.FileSizeCtx).(uint64)
	if !ok {
		return serializer.UploadCredential{}, errors.New("無法獲取檔案大小")
	}

	// 如果小於4MB，則由服務端中轉
	if fileSize <= SmallFileSize {
		return serializer.UploadCredential{}, nil
	}

	// 生成回調地址
	siteURL := model.GetSiteURL()
	apiBaseURI, _ := url.Parse("/api/v3/callback/onedrive/finish/" + key)
	apiURL := siteURL.ResolveReference(apiBaseURI)

	uploadURL, err := handler.Client.CreateUploadSession(ctx, savePath, WithConflictBehavior("fail"))
	if err != nil {
		return serializer.UploadCredential{}, err
	}

	// 監控回調及上傳
	go handler.Client.MonitorUpload(uploadURL, key, savePath, fileSize, TTL)

	return serializer.UploadCredential{
		Policy: uploadURL,
		Token:  apiURL.String(),
	}, nil
}
