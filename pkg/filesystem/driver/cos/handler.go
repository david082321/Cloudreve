package cos

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
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
	"github.com/google/go-querystring/query"
	cossdk "github.com/tencentyun/cos-go-sdk-v5"
)

// UploadPolicy 騰訊雲COS上傳策略
type UploadPolicy struct {
	Expiration string        `json:"expiration"`
	Conditions []interface{} `json:"conditions"`
}

// MetaData 文件元訊息
type MetaData struct {
	Size        uint64
	CallbackKey string
	CallbackURL string
}

type urlOption struct {
	Speed              int    `url:"x-cos-traffic-limit,omitempty"`
	ContentDescription string `url:"response-content-disposition,omitempty"`
}

// Driver 騰訊雲COS適配器模板
type Driver struct {
	Policy     *model.Policy
	Client     *cossdk.Client
	HTTPClient request.Client
}

// List 列出COS文件
func (handler Driver) List(ctx context.Context, base string, recursive bool) ([]response.Object, error) {
	// 初始化列目錄參數
	opt := &cossdk.BucketGetOptions{
		Prefix:       strings.TrimPrefix(base, "/"),
		EncodingType: "",
		MaxKeys:      1000,
	}
	// 是否為遞迴列出
	if !recursive {
		opt.Delimiter = "/"
	}
	// 手動補齊結尾的slash
	if opt.Prefix != "" {
		opt.Prefix += "/"
	}

	var (
		marker  string
		objects []cossdk.Object
		commons []string
	)

	for {
		res, _, err := handler.Client.Bucket.Get(ctx, opt)
		if err != nil {
			return nil, err
		}
		objects = append(objects, res.Contents...)
		commons = append(commons, res.CommonPrefixes...)
		// 如果本次未列取完，則繼續使用marker獲取結果
		marker = res.NextMarker
		// marker 為空時結果列取完畢，跳出
		if marker == "" {
			break
		}
	}

	// 處理列取結果
	res := make([]response.Object, 0, len(objects)+len(commons))
	// 處理目錄
	for _, object := range commons {
		rel, err := filepath.Rel(opt.Prefix, object)
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
		rel, err := filepath.Rel(opt.Prefix, object.Key)
		if err != nil {
			continue
		}
		res = append(res, response.Object{
			Name:         path.Base(object.Key),
			Source:       object.Key,
			RelativePath: filepath.ToSlash(rel),
			Size:         uint64(object.Size),
			IsDir:        false,
			LastModify:   time.Now(),
		})
	}

	return res, nil

}

// CORS 建立跨域策略
func (handler Driver) CORS() error {
	_, err := handler.Client.Bucket.PutCORS(context.Background(), &cossdk.BucketPutCORSOptions{
		Rules: []cossdk.BucketCORSRule{{
			AllowedMethods: []string{
				"GET",
				"POST",
				"PUT",
				"DELETE",
				"HEAD",
			},
			AllowedOrigins: []string{"*"},
			AllowedHeaders: []string{"*"},
			MaxAgeSeconds:  3600,
			ExposeHeaders:  []string{},
		}},
	})

	return err
}

// Get 獲取文件
func (handler Driver) Get(ctx context.Context, path string) (response.RSCloser, error) {
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
	opt := &cossdk.ObjectPutOptions{}
	_, err := handler.Client.Object.Put(ctx, dst, file, opt)
	return err
}

// Delete 刪除一個或多個文件，
// 返回未刪除的文件，及遇到的最後一個錯誤
func (handler Driver) Delete(ctx context.Context, files []string) ([]string, error) {
	obs := []cossdk.Object{}
	for _, v := range files {
		obs = append(obs, cossdk.Object{Key: v})
	}
	opt := &cossdk.ObjectDeleteMultiOptions{
		Objects: obs,
		Quiet:   true,
	}

	res, _, err := handler.Client.Object.DeleteMulti(context.Background(), opt)
	if err != nil {
		return files, err
	}

	// 整理刪除結果
	failed := make([]string, 0, len(files))
	for _, v := range res.Errors {
		failed = append(failed, v.Key)
	}

	if len(failed) == 0 {
		return failed, nil
	}

	return failed, errors.New("刪除失敗")
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
	thumbParam := fmt.Sprintf("imageMogr2/thumbnail/%dx%d", thumbSize[0], thumbSize[1])

	source, err := handler.signSourceURL(
		ctx,
		path,
		int64(model.GetIntSetting("preview_timeout", 60)),
		&urlOption{},
	)
	if err != nil {
		return nil, err
	}

	thumbURL, _ := url.Parse(source)
	thumbQuery := thumbURL.Query()
	thumbQuery.Add(thumbParam, "")
	thumbURL.RawQuery = thumbQuery.Encode()

	return &response.ContentResponse{
		Redirect: true,
		URL:      thumbURL.String(),
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

	// 添加各項設定
	options := urlOption{}
	if speed > 0 {
		if speed < 819200 {
			speed = 819200
		}
		if speed > 838860800 {
			speed = 838860800
		}
		options.Speed = speed
	}
	if isDownload {
		options.ContentDescription = "attachment; filename=\"" + url.PathEscape(fileName) + "\""
	}

	return handler.signSourceURL(ctx, path, ttl, &options)
}

func (handler Driver) signSourceURL(ctx context.Context, path string, ttl int64, options *urlOption) (string, error) {
	cdnURL, err := url.Parse(handler.Policy.BaseURL)
	if err != nil {
		return "", err
	}

	// 公有空間不需要簽名
	if !handler.Policy.IsPrivate {
		file, err := url.Parse(path)
		if err != nil {
			return "", err
		}

		// 非簽名URL不支援設定響應header
		options.ContentDescription = ""

		optionQuery, err := query.Values(*options)
		if err != nil {
			return "", err
		}
		file.RawQuery = optionQuery.Encode()
		sourceURL := cdnURL.ResolveReference(file)

		return sourceURL.String(), nil
	}

	presignedURL, err := handler.Client.Object.GetPresignedURL(ctx, http.MethodGet, path,
		handler.Policy.AccessKey, handler.Policy.SecretKey, time.Duration(ttl)*time.Second, options)
	if err != nil {
		return "", err
	}

	// 將最終生成的簽名URL域名換成使用者自訂的加速域名（如果有）
	presignedURL.Host = cdnURL.Host
	presignedURL.Scheme = cdnURL.Scheme

	return presignedURL.String(), nil
}

// Token 獲取上傳策略和認證Token
func (handler Driver) Token(ctx context.Context, TTL int64, key string) (serializer.UploadCredential, error) {
	// 讀取上下文中生成的儲存路徑
	savePath, ok := ctx.Value(fsctx.SavePathCtx).(string)
	if !ok {
		return serializer.UploadCredential{}, errors.New("無法獲取儲存路徑")
	}

	// 生成回調地址
	siteURL := model.GetSiteURL()
	apiBaseURI, _ := url.Parse("/api/v3/callback/cos/" + key)
	apiURL := siteURL.ResolveReference(apiBaseURI).String()

	// 上傳策略
	startTime := time.Now()
	endTime := startTime.Add(time.Duration(TTL) * time.Second)
	keyTime := fmt.Sprintf("%d;%d", startTime.Unix(), endTime.Unix())
	postPolicy := UploadPolicy{
		Expiration: endTime.UTC().Format(time.RFC3339),
		Conditions: []interface{}{
			map[string]string{"bucket": handler.Policy.BucketName},
			map[string]string{"$key": savePath},
			map[string]string{"x-cos-meta-callback": apiURL},
			map[string]string{"x-cos-meta-key": key},
			map[string]string{"q-sign-algorithm": "sha1"},
			map[string]string{"q-ak": handler.Policy.AccessKey},
			map[string]string{"q-sign-time": keyTime},
		},
	}

	if handler.Policy.MaxSize > 0 {
		postPolicy.Conditions = append(postPolicy.Conditions,
			[]interface{}{"content-length-range", 0, handler.Policy.MaxSize})
	}

	res, err := handler.getUploadCredential(ctx, postPolicy, keyTime)
	if err == nil {
		res.Callback = apiURL
		res.Key = key
	}

	return res, err

}

// Meta 獲取文件訊息
func (handler Driver) Meta(ctx context.Context, path string) (*MetaData, error) {
	res, err := handler.Client.Object.Head(ctx, path, &cossdk.ObjectHeadOptions{})
	if err != nil {
		return nil, err
	}
	return &MetaData{
		Size:        uint64(res.ContentLength),
		CallbackKey: res.Header.Get("x-cos-meta-key"),
		CallbackURL: res.Header.Get("x-cos-meta-callback"),
	}, nil
}

func (handler Driver) getUploadCredential(ctx context.Context, policy UploadPolicy, keyTime string) (serializer.UploadCredential, error) {
	// 讀取上下文中生成的儲存路徑
	savePath, ok := ctx.Value(fsctx.SavePathCtx).(string)
	if !ok {
		return serializer.UploadCredential{}, errors.New("無法獲取儲存路徑")
	}

	// 編碼上傳策略
	policyJSON, err := json.Marshal(policy)
	if err != nil {
		return serializer.UploadCredential{}, err
	}
	policyEncoded := base64.StdEncoding.EncodeToString(policyJSON)

	// 簽名上傳策略
	hmacSign := hmac.New(sha1.New, []byte(handler.Policy.SecretKey))
	_, err = io.WriteString(hmacSign, keyTime)
	if err != nil {
		return serializer.UploadCredential{}, err
	}
	signKey := fmt.Sprintf("%x", hmacSign.Sum(nil))

	sha1Sign := sha1.New()
	_, err = sha1Sign.Write(policyJSON)
	if err != nil {
		return serializer.UploadCredential{}, err
	}
	stringToSign := fmt.Sprintf("%x", sha1Sign.Sum(nil))

	// 最終簽名
	hmacFinalSign := hmac.New(sha1.New, []byte(signKey))
	_, err = hmacFinalSign.Write([]byte(stringToSign))
	if err != nil {
		return serializer.UploadCredential{}, err
	}
	signature := hmacFinalSign.Sum(nil)

	return serializer.UploadCredential{
		Policy:    policyEncoded,
		Path:      savePath,
		AccessKey: handler.Policy.AccessKey,
		Token:     fmt.Sprintf("%x", signature),
		KeyTime:   keyTime,
	}, nil
}
