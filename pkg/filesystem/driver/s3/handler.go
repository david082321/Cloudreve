package s3

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/cloudreve/Cloudreve/v3/pkg/util"
	"io"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	model "github.com/cloudreve/Cloudreve/v3/models"
	"github.com/cloudreve/Cloudreve/v3/pkg/filesystem/fsctx"
	"github.com/cloudreve/Cloudreve/v3/pkg/filesystem/response"
	"github.com/cloudreve/Cloudreve/v3/pkg/request"
	"github.com/cloudreve/Cloudreve/v3/pkg/serializer"
)

// Driver 適配器模板
type Driver struct {
	Policy *model.Policy
	sess   *session.Session
	svc    *s3.S3
}

// UploadPolicy S3上傳策略
type UploadPolicy struct {
	Expiration string        `json:"expiration"`
	Conditions []interface{} `json:"conditions"`
}

//MetaData 文件訊息
type MetaData struct {
	Size uint64
	Etag string
}

// InitS3Client 初始化S3工作階段
func (handler *Driver) InitS3Client() error {
	if handler.Policy == nil {
		return errors.New("儲存策略為空")
	}

	if handler.svc == nil {
		// 初始化工作階段
		sess, err := session.NewSession(&aws.Config{
			Credentials:      credentials.NewStaticCredentials(handler.Policy.AccessKey, handler.Policy.SecretKey, ""),
			Endpoint:         &handler.Policy.Server,
			Region:           &handler.Policy.OptionsSerialized.Region,
			S3ForcePathStyle: aws.Bool(true),
		})

		if err != nil {
			return err
		}
		handler.sess = sess
		handler.svc = s3.New(sess)
	}
	return nil
}

// List 列出給定路徑下的文件
func (handler Driver) List(ctx context.Context, base string, recursive bool) ([]response.Object, error) {

	// 初始化用戶端
	if err := handler.InitS3Client(); err != nil {
		return nil, err
	}

	// 初始化列目錄參數
	base = strings.TrimPrefix(base, "/")
	if base != "" {
		base += "/"
	}

	opt := &s3.ListObjectsInput{
		Bucket:  &handler.Policy.BucketName,
		Prefix:  &base,
		MaxKeys: aws.Int64(1000),
	}

	// 是否為遞迴列出
	if !recursive {
		opt.Delimiter = aws.String("/")
	}

	var (
		objects []*s3.Object
		commons []*s3.CommonPrefix
	)

	for {
		res, err := handler.svc.ListObjectsWithContext(ctx, opt)
		if err != nil {
			return nil, err
		}
		objects = append(objects, res.Contents...)
		commons = append(commons, res.CommonPrefixes...)

		// 如果本次未列取完，則繼續使用marker獲取結果
		if *res.IsTruncated {
			opt.Marker = res.NextMarker
		} else {
			break
		}
	}

	// 處理列取結果
	res := make([]response.Object, 0, len(objects)+len(commons))

	// 處理目錄
	for _, object := range commons {
		rel, err := filepath.Rel(*opt.Prefix, *object.Prefix)
		if err != nil {
			continue
		}
		res = append(res, response.Object{
			Name:         path.Base(*object.Prefix),
			RelativePath: filepath.ToSlash(rel),
			Size:         0,
			IsDir:        true,
			LastModify:   time.Now(),
		})
	}
	// 處理文件
	for _, object := range objects {
		rel, err := filepath.Rel(*opt.Prefix, *object.Key)
		if err != nil {
			continue
		}
		res = append(res, response.Object{
			Name:         path.Base(*object.Key),
			Source:       *object.Key,
			RelativePath: filepath.ToSlash(rel),
			Size:         uint64(*object.Size),
			IsDir:        false,
			LastModify:   time.Now(),
		})
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

	// 初始化用戶端
	if err := handler.InitS3Client(); err != nil {
		return err
	}

	uploader := s3manager.NewUploader(handler.sess)

	_, err := uploader.Upload(&s3manager.UploadInput{
		Bucket: &handler.Policy.BucketName,
		Key:    &dst,
		Body:   file,
	})

	if err != nil {
		return err
	}

	return nil
}

// Delete 刪除一個或多個文件，
// 返回未刪除的文件，及遇到的最後一個錯誤
func (handler Driver) Delete(ctx context.Context, files []string) ([]string, error) {

	// 初始化用戶端
	if err := handler.InitS3Client(); err != nil {
		return files, err
	}

	failed := make([]string, 0, len(files))
	deleted := make([]string, 0, len(files))

	keys := make([]*s3.ObjectIdentifier, 0, len(files))
	for _, file := range files {
		filePath := file
		keys = append(keys, &s3.ObjectIdentifier{Key: &filePath})
	}

	// 發送非同步刪除請求
	res, err := handler.svc.DeleteObjects(
		&s3.DeleteObjectsInput{
			Bucket: &handler.Policy.BucketName,
			Delete: &s3.Delete{
				Objects: keys,
			},
		})

	if err != nil {
		return files, err
	}

	// 統計未刪除的文件
	for _, deleteRes := range res.Deleted {
		deleted = append(deleted, *deleteRes.Key)
	}
	failed = util.SliceDifference(failed, deleted)

	return failed, nil

}

// Thumb 獲取文件縮圖
func (handler Driver) Thumb(ctx context.Context, path string) (*response.ContentResponse, error) {
	return nil, errors.New("未實現")
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

	// 初始化用戶端
	if err := handler.InitS3Client(); err != nil {
		return "", err
	}

	req, _ := handler.svc.GetObjectRequest(
		&s3.GetObjectInput{
			Bucket:                     &handler.Policy.BucketName,
			Key:                        &path,
			ResponseContentDisposition: aws.String("attachment; filename=\"" + url.PathEscape(fileName) + "\""),
		})

	if ttl == 0 {
		ttl = 3600
	}

	signedURL, _ := req.Presign(time.Duration(ttl) * time.Second)

	// 將最終生成的簽名URL域名換成使用者自訂的加速域名（如果有）
	finalURL, err := url.Parse(signedURL)
	if err != nil {
		return "", err
	}

	// 公有空間取代掉Key及不支援的頭
	if !handler.Policy.IsPrivate {
		finalURL.RawQuery = ""
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

	// 讀取上下文中生成的儲存路徑和檔案大小
	savePath, ok := ctx.Value(fsctx.SavePathCtx).(string)
	if !ok {
		return serializer.UploadCredential{}, errors.New("無法獲取儲存路徑")
	}

	// 生成回調地址
	siteURL := model.GetSiteURL()
	apiBaseURI, _ := url.Parse("/api/v3/callback/s3/" + key)
	apiURL := siteURL.ResolveReference(apiBaseURI)

	// 上傳策略
	putPolicy := UploadPolicy{
		Expiration: time.Now().UTC().Add(time.Duration(TTL) * time.Second).Format(time.RFC3339),
		Conditions: []interface{}{
			map[string]string{"bucket": handler.Policy.BucketName},
			[]string{"starts-with", "$key", savePath},
			[]string{"starts-with", "$success_action_redirect", apiURL.String()},
			[]string{"starts-with", "$Content-Type", ""},
			map[string]string{"x-amz-algorithm": "AWS4-HMAC-SHA256"},
		},
	}

	if handler.Policy.MaxSize > 0 {
		putPolicy.Conditions = append(putPolicy.Conditions,
			[]interface{}{"content-length-range", 0, handler.Policy.MaxSize})
	}

	// 生成上傳憑證
	return handler.getUploadCredential(ctx, putPolicy, apiURL)
}

// Meta 獲取文件訊息
func (handler Driver) Meta(ctx context.Context, path string) (*MetaData, error) {
	// 初始化用戶端
	if err := handler.InitS3Client(); err != nil {
		return nil, err
	}

	res, err := handler.svc.GetObject(
		&s3.GetObjectInput{
			Bucket: &handler.Policy.BucketName,
			Key:    &path,
		})

	if err != nil {
		return nil, err
	}

	return &MetaData{
		Size: uint64(*res.ContentLength),
		Etag: *res.ETag,
	}, nil

}

func (handler Driver) getUploadCredential(ctx context.Context, policy UploadPolicy, callback *url.URL) (serializer.UploadCredential, error) {

	// 讀取上下文中生成的儲存路徑和檔案大小
	savePath, ok := ctx.Value(fsctx.SavePathCtx).(string)
	if !ok {
		return serializer.UploadCredential{}, errors.New("無法獲取儲存路徑")
	}

	longDate := time.Now().UTC().Format("20060102T150405Z")
	shortDate := time.Now().UTC().Format("20060102")

	credential := handler.Policy.AccessKey + "/" + shortDate + "/" + handler.Policy.OptionsSerialized.Region + "/s3/aws4_request"
	policy.Conditions = append(policy.Conditions, map[string]string{"x-amz-credential": credential})
	policy.Conditions = append(policy.Conditions, map[string]string{"x-amz-date": longDate})

	// 編碼上傳策略
	policyJSON, err := json.Marshal(policy)
	if err != nil {
		return serializer.UploadCredential{}, err
	}
	policyEncoded := base64.StdEncoding.EncodeToString(policyJSON)

	//簽名
	signature := getHMAC([]byte("AWS4"+handler.Policy.SecretKey), []byte(shortDate))
	signature = getHMAC(signature, []byte(handler.Policy.OptionsSerialized.Region))
	signature = getHMAC(signature, []byte("s3"))
	signature = getHMAC(signature, []byte("aws4_request"))
	signature = getHMAC(signature, []byte(policyEncoded))

	return serializer.UploadCredential{
		Policy:    policyEncoded,
		Callback:  callback.String(),
		Token:     hex.EncodeToString(signature),
		AccessKey: credential,
		Path:      savePath,
		KeyTime:   longDate,
	}, nil
}

func getHMAC(key []byte, data []byte) []byte {
	hash := hmac.New(sha256.New, key)
	hash.Write(data)
	return hash.Sum(nil)
}

// CORS 建立跨域策略
func (handler Driver) CORS() error {
	// 初始化用戶端
	if err := handler.InitS3Client(); err != nil {
		return err
	}

	rule := s3.CORSRule{
		AllowedMethods: aws.StringSlice([]string{
			"GET",
			"POST",
			"PUT",
			"DELETE",
			"HEAD",
		}),
		AllowedOrigins: aws.StringSlice([]string{"*"}),
		AllowedHeaders: aws.StringSlice([]string{"*"}),
		MaxAgeSeconds:  aws.Int64(3600),
	}

	_, err := handler.svc.PutBucketCors(&s3.PutBucketCorsInput{
		Bucket: &handler.Policy.BucketName,
		CORSConfiguration: &s3.CORSConfiguration{
			CORSRules: []*s3.CORSRule{&rule},
		},
	})

	return err
}
