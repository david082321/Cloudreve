package oss

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	model "github.com/cloudreve/Cloudreve/v3/models"
	"github.com/cloudreve/Cloudreve/v3/pkg/filesystem/fsctx"
	"github.com/cloudreve/Cloudreve/v3/pkg/filesystem/response"
	"github.com/cloudreve/Cloudreve/v3/pkg/request"
	"github.com/cloudreve/Cloudreve/v3/pkg/serializer"
	"github.com/cloudreve/Cloudreve/v3/pkg/util"
)

// UploadPolicy 阿里雲OSS上傳策略
type UploadPolicy struct {
	Expiration string        `json:"expiration"`
	Conditions []interface{} `json:"conditions"`
}

// CallbackPolicy 回調策略
type CallbackPolicy struct {
	CallbackURL      string `json:"callbackUrl"`
	CallbackBody     string `json:"callbackBody"`
	CallbackBodyType string `json:"callbackBodyType"`
}

// Driver 阿里雲OSS策略適配器
type Driver struct {
	Policy     *model.Policy
	client     *oss.Client
	bucket     *oss.Bucket
	HTTPClient request.Client
}

type key int

const (
	// VersionID 文件版本標識
	VersionID key = iota
)

// CORS 建立跨域策略
func (handler *Driver) CORS() error {
	// 初始化用戶端
	if err := handler.InitOSSClient(false); err != nil {
		return err
	}

	return handler.client.SetBucketCORS(handler.Policy.BucketName, []oss.CORSRule{
		{
			AllowedOrigin: []string{"*"},
			AllowedMethod: []string{
				"GET",
				"POST",
				"PUT",
				"DELETE",
				"HEAD",
			},
			ExposeHeader:  []string{},
			AllowedHeader: []string{"*"},
			MaxAgeSeconds: 3600,
		},
	})
}

// InitOSSClient 初始化OSS鑒權用戶端
func (handler *Driver) InitOSSClient(forceUsePublicEndpoint bool) error {
	if handler.Policy == nil {
		return errors.New("儲存策略為空")
	}

	if handler.client == nil {
		// 決定是否使用內網 Endpoint
		endpoint := handler.Policy.Server
		if handler.Policy.OptionsSerialized.ServerSideEndpoint != "" && !forceUsePublicEndpoint {
			endpoint = handler.Policy.OptionsSerialized.ServerSideEndpoint
		}

		// 初始化用戶端
		client, err := oss.New(endpoint, handler.Policy.AccessKey, handler.Policy.SecretKey)
		if err != nil {
			return err
		}
		handler.client = client

		// 初始化儲存桶
		bucket, err := client.Bucket(handler.Policy.BucketName)
		if err != nil {
			return err
		}
		handler.bucket = bucket

	}

	return nil
}

// List 列出OSS上的文件
func (handler Driver) List(ctx context.Context, base string, recursive bool) ([]response.Object, error) {
	// 初始化用戶端
	if err := handler.InitOSSClient(false); err != nil {
		return nil, err
	}

	// 列取文件
	base = strings.TrimPrefix(base, "/")
	if base != "" {
		base += "/"
	}

	var (
		delimiter string
		marker    string
		objects   []oss.ObjectProperties
		commons   []string
	)
	if !recursive {
		delimiter = "/"
	}

	for {
		subRes, err := handler.bucket.ListObjects(oss.Marker(marker), oss.Prefix(base),
			oss.MaxKeys(1000), oss.Delimiter(delimiter))
		if err != nil {
			return nil, err
		}
		objects = append(objects, subRes.Objects...)
		commons = append(commons, subRes.CommonPrefixes...)
		marker = subRes.NextMarker
		if marker == "" {
			break
		}
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
			Size:         uint64(object.Size),
			IsDir:        false,
			LastModify:   object.LastModified,
		})
	}

	return res, nil
}

// Get 獲取文件
func (handler Driver) Get(ctx context.Context, path string) (response.RSCloser, error) {
	// 透過VersionID禁止快取
	ctx = context.WithValue(ctx, VersionID, time.Now().UnixNano())

	// 儘可能使用私有 Endpoint
	ctx = context.WithValue(ctx, fsctx.ForceUsePublicEndpointCtx, false)

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
	defer file.Close()

	// 初始化用戶端
	if err := handler.InitOSSClient(false); err != nil {
		return err
	}

	// 憑證有效期
	credentialTTL := model.GetIntSetting("upload_credential_timeout", 3600)

	// 是否允許覆蓋
	overwrite := true
	if ctx.Value(fsctx.DisableOverwrite) != nil {
		overwrite = false
	}

	options := []oss.Option{
		oss.Expires(time.Now().Add(time.Duration(credentialTTL) * time.Second)),
		oss.ForbidOverWrite(!overwrite),
	}

	// 上傳文件
	err := handler.bucket.PutObject(dst, file, options...)
	if err != nil {
		return err
	}

	return nil
}

// Delete 刪除一個或多個文件，
// 返回未刪除的文件
func (handler Driver) Delete(ctx context.Context, files []string) ([]string, error) {
	// 初始化用戶端
	if err := handler.InitOSSClient(false); err != nil {
		return files, err
	}

	// 刪除文件
	delRes, err := handler.bucket.DeleteObjects(files)

	if err != nil {
		return files, err
	}

	// 統計未刪除的文件
	failed := util.SliceDifference(files, delRes.DeletedObjects)
	if len(failed) > 0 {
		return failed, errors.New("刪除失敗")
	}

	return []string{}, nil
}

// Thumb 獲取文件縮圖
func (handler Driver) Thumb(ctx context.Context, path string) (*response.ContentResponse, error) {
	// 初始化用戶端
	if err := handler.InitOSSClient(true); err != nil {
		return nil, err
	}

	var (
		thumbSize = [2]uint{400, 300}
		ok        = false
	)
	if thumbSize, ok = ctx.Value(fsctx.ThumbSizeCtx).([2]uint); !ok {
		return nil, errors.New("無法獲取縮圖尺寸設定")
	}

	thumbParam := fmt.Sprintf("image/resize,m_lfit,h_%d,w_%d", thumbSize[1], thumbSize[0])
	ctx = context.WithValue(ctx, fsctx.ThumbSizeCtx, thumbParam)
	thumbOption := []oss.Option{oss.Process(thumbParam)}
	thumbURL, err := handler.signSourceURL(
		ctx,
		path,
		int64(model.GetIntSetting("preview_timeout", 60)),
		thumbOption,
	)
	if err != nil {
		return nil, err
	}

	return &response.ContentResponse{
		Redirect: true,
		URL:      thumbURL,
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
	// 初始化用戶端
	usePublicEndpoint := true
	if forceUsePublicEndpoint, ok := ctx.Value(fsctx.ForceUsePublicEndpointCtx).(bool); ok {
		usePublicEndpoint = forceUsePublicEndpoint
	}
	if err := handler.InitOSSClient(usePublicEndpoint); err != nil {
		return "", err
	}

	// 嘗試從上下文獲取檔案名
	fileName := ""
	if file, ok := ctx.Value(fsctx.FileModelCtx).(model.File); ok {
		fileName = file.Name
	}

	// 添加各項設定
	var signOptions = make([]oss.Option, 0, 2)
	if isDownload {
		signOptions = append(signOptions, oss.ResponseContentDisposition("attachment; filename=\""+url.PathEscape(fileName)+"\""))
	}
	if speed > 0 {
		// Byte 轉換為 bit
		speed *= 8

		// OSS對速度值有範圍限制
		if speed < 819200 {
			speed = 819200
		}
		if speed > 838860800 {
			speed = 838860800
		}
		signOptions = append(signOptions, oss.TrafficLimitParam(int64(speed)))
	}

	return handler.signSourceURL(ctx, path, ttl, signOptions)
}

func (handler Driver) signSourceURL(ctx context.Context, path string, ttl int64, options []oss.Option) (string, error) {
	signedURL, err := handler.bucket.SignURL(path, oss.HTTPGet, ttl, options...)
	if err != nil {
		return "", err
	}

	// 將最終生成的簽名URL域名換成使用者自訂的加速域名（如果有）
	finalURL, err := url.Parse(signedURL)
	if err != nil {
		return "", err
	}

	// 優先使用https
	finalURL.Scheme = "https"

	// 公有空間取代掉Key及不支援的頭
	if !handler.Policy.IsPrivate {
		query := finalURL.Query()
		query.Del("OSSAccessKeyId")
		query.Del("Signature")
		query.Del("response-content-disposition")
		query.Del("x-oss-traffic-limit")
		finalURL.RawQuery = query.Encode()
	}

	if handler.Policy.BaseURL != "" {
		cdnURL, err := url.Parse(handler.Policy.BaseURL)
		if err != nil {
			return "", err
		}
		finalURL.Host = cdnURL.Host
		finalURL.Scheme = cdnURL.Scheme
	}

	return finalURL.String(), nil
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
	apiBaseURI, _ := url.Parse("/api/v3/callback/oss/" + key)
	apiURL := siteURL.ResolveReference(apiBaseURI)

	// 回調策略
	callbackPolicy := CallbackPolicy{
		CallbackURL:      apiURL.String(),
		CallbackBody:     `{"name":${x:fname},"source_name":${object},"size":${size},"pic_info":"${imageInfo.width},${imageInfo.height}"}`,
		CallbackBodyType: "application/json",
	}

	// 上傳策略
	postPolicy := UploadPolicy{
		Expiration: time.Now().UTC().Add(time.Duration(TTL) * time.Second).Format(time.RFC3339),
		Conditions: []interface{}{
			map[string]string{"bucket": handler.Policy.BucketName},
			[]string{"starts-with", "$key", path.Dir(savePath)},
		},
	}

	if handler.Policy.MaxSize > 0 {
		postPolicy.Conditions = append(postPolicy.Conditions,
			[]interface{}{"content-length-range", 0, handler.Policy.MaxSize})
	}

	return handler.getUploadCredential(ctx, postPolicy, callbackPolicy, TTL)
}

func (handler Driver) getUploadCredential(ctx context.Context, policy UploadPolicy, callback CallbackPolicy, TTL int64) (serializer.UploadCredential, error) {
	// 讀取上下文中生成的儲存路徑
	savePath, ok := ctx.Value(fsctx.SavePathCtx).(string)
	if !ok {
		return serializer.UploadCredential{}, errors.New("無法獲取儲存路徑")
	}

	// 處理回調策略
	callbackPolicyEncoded := ""
	if callback.CallbackURL != "" {
		callbackPolicyJSON, err := json.Marshal(callback)
		if err != nil {
			return serializer.UploadCredential{}, err
		}
		callbackPolicyEncoded = base64.StdEncoding.EncodeToString(callbackPolicyJSON)
		policy.Conditions = append(policy.Conditions, map[string]string{"callback": callbackPolicyEncoded})
	}

	// 編碼上傳策略
	policyJSON, err := json.Marshal(policy)
	if err != nil {
		return serializer.UploadCredential{}, err
	}
	policyEncoded := base64.StdEncoding.EncodeToString(policyJSON)

	// 簽名上傳策略
	hmacSign := hmac.New(sha1.New, []byte(handler.Policy.SecretKey))
	_, err = io.WriteString(hmacSign, policyEncoded)
	if err != nil {
		return serializer.UploadCredential{}, err
	}
	signature := base64.StdEncoding.EncodeToString(hmacSign.Sum(nil))

	return serializer.UploadCredential{
		Policy:    fmt.Sprintf("%s:%s", callbackPolicyEncoded, policyEncoded),
		Path:      savePath,
		AccessKey: handler.Policy.AccessKey,
		Token:     signature,
	}, nil
}
