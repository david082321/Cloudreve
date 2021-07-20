package upyun

import (
	"context"
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	model "github.com/cloudreve/Cloudreve/v3/models"
	"github.com/cloudreve/Cloudreve/v3/pkg/filesystem/fsctx"
	"github.com/cloudreve/Cloudreve/v3/pkg/filesystem/response"
	"github.com/cloudreve/Cloudreve/v3/pkg/request"
	"github.com/cloudreve/Cloudreve/v3/pkg/serializer"
	"github.com/upyun/go-sdk/upyun"
)

// UploadPolicy 又拍雲上傳策略
type UploadPolicy struct {
	Bucket             string `json:"bucket"`
	SaveKey            string `json:"save-key"`
	Expiration         int64  `json:"expiration"`
	CallbackURL        string `json:"notify-url"`
	ContentLength      uint64 `json:"content-length"`
	ContentLengthRange string `json:"content-length-range,omitempty"`
	AllowFileType      string `json:"allow-file-type,omitempty"`
}

// Driver 又拍雲策略適配器
type Driver struct {
	Policy *model.Policy
}

func (handler Driver) List(ctx context.Context, base string, recursive bool) ([]response.Object, error) {
	base = strings.TrimPrefix(base, "/")

	// 用於接受SDK返回物件的chan
	objChan := make(chan *upyun.FileInfo)
	objects := []*upyun.FileInfo{}

	// 列取配置
	listConf := &upyun.GetObjectsConfig{
		Path:         "/" + base,
		ObjectsChan:  objChan,
		MaxListTries: 1,
	}
	// 遞迴列取時不限制遞迴次數
	if recursive {
		listConf.MaxListLevel = -1
	}

	// 啟動一個goroutine收集列取的物件信
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func(input chan *upyun.FileInfo, output *[]*upyun.FileInfo, wg *sync.WaitGroup) {
		defer wg.Done()
		for {
			file, ok := <-input
			if !ok {
				return
			}
			*output = append(*output, file)
		}
	}(objChan, &objects, wg)

	up := upyun.NewUpYun(&upyun.UpYunConfig{
		Bucket:   handler.Policy.BucketName,
		Operator: handler.Policy.AccessKey,
		Password: handler.Policy.SecretKey,
	})

	err := up.List(listConf)
	if err != nil {
		return nil, err
	}

	wg.Wait()

	// 匯總處理列取結果
	res := make([]response.Object, 0, len(objects))
	for _, object := range objects {
		res = append(res, response.Object{
			Name:         path.Base(object.Name),
			RelativePath: object.Name,
			Source:       path.Join(base, object.Name),
			Size:         uint64(object.Size),
			IsDir:        object.IsDir,
			LastModify:   object.Time,
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
	defer file.Close()

	up := upyun.NewUpYun(&upyun.UpYunConfig{
		Bucket:   handler.Policy.BucketName,
		Operator: handler.Policy.AccessKey,
		Password: handler.Policy.SecretKey,
	})
	err := up.Put(&upyun.PutObjectConfig{
		Path:   dst,
		Reader: file,
	})

	return err
}

// Delete 刪除一個或多個文件，
// 返回未刪除的文件，及遇到的最後一個錯誤
func (handler Driver) Delete(ctx context.Context, files []string) ([]string, error) {
	up := upyun.NewUpYun(&upyun.UpYunConfig{
		Bucket:   handler.Policy.BucketName,
		Operator: handler.Policy.AccessKey,
		Password: handler.Policy.SecretKey,
	})

	var (
		failed       = make([]string, 0, len(files))
		lastErr      error
		currentIndex = 0
		indexLock    sync.Mutex
		failedLock   sync.Mutex
		wg           sync.WaitGroup
		routineNum   = 4
	)
	wg.Add(routineNum)

	// upyun不支援批次操作，這裡開四個協程並行操作
	for i := 0; i < routineNum; i++ {
		go func() {
			for {
				// 取得待刪除文件
				indexLock.Lock()
				if currentIndex >= len(files) {
					// 所有文件處理完成
					wg.Done()
					indexLock.Unlock()
					return
				}
				path := files[currentIndex]
				currentIndex++
				indexLock.Unlock()

				// 發送非同步刪除請求
				err := up.Delete(&upyun.DeleteObjectConfig{
					Path:  path,
					Async: true,
				})

				// 處理錯誤
				if err != nil {
					failedLock.Lock()
					lastErr = err
					failed = append(failed, path)
					failedLock.Unlock()
				}
			}
		}()
	}

	wg.Wait()

	return failed, lastErr
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

	thumbParam := fmt.Sprintf("!/fwfh/%dx%d", thumbSize[0], thumbSize[1])
	thumbURL, err := handler.Source(
		ctx,
		path+thumbParam,
		url.URL{},
		int64(model.GetIntSetting("preview_timeout", 60)),
		false,
		0,
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
	// 嘗試從上下文獲取檔案名
	fileName := ""
	if file, ok := ctx.Value(fsctx.FileModelCtx).(model.File); ok {
		fileName = file.Name
	}

	sourceURL, err := url.Parse(handler.Policy.BaseURL)
	if err != nil {
		return "", err
	}

	fileKey, err := url.Parse(url.PathEscape(path))
	if err != nil {
		return "", err
	}

	sourceURL = sourceURL.ResolveReference(fileKey)

	// 如果是下載文件URL
	if isDownload {
		query := sourceURL.Query()
		query.Add("_upd", fileName)
		sourceURL.RawQuery = query.Encode()
	}

	return handler.signURL(ctx, sourceURL, ttl)
}

func (handler Driver) signURL(ctx context.Context, path *url.URL, TTL int64) (string, error) {
	if !handler.Policy.IsPrivate {
		// 未開啟Token防盜鏈時，直接返回
		return path.String(), nil
	}

	etime := time.Now().Add(time.Duration(TTL) * time.Second).Unix()
	signStr := fmt.Sprintf(
		"%s&%d&%s",
		handler.Policy.OptionsSerialized.Token,
		etime,
		path.Path,
	)
	signMd5 := fmt.Sprintf("%x", md5.Sum([]byte(signStr)))
	finalSign := signMd5[12:20] + strconv.FormatInt(etime, 10)

	// 將簽名添加到URL中
	query := path.Query()
	query.Add("_upt", finalSign)
	path.RawQuery = query.Encode()

	return path.String(), nil
}

// Token 獲取上傳策略和認證Token
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

	// 檢查檔案大小

	// 生成回調地址
	siteURL := model.GetSiteURL()
	apiBaseURI, _ := url.Parse("/api/v3/callback/upyun/" + key)
	apiURL := siteURL.ResolveReference(apiBaseURI)

	// 上傳策略
	putPolicy := UploadPolicy{
		Bucket: handler.Policy.BucketName,
		// TODO escape
		SaveKey:            savePath,
		Expiration:         time.Now().Add(time.Duration(TTL) * time.Second).Unix(),
		CallbackURL:        apiURL.String(),
		ContentLength:      fileSize,
		ContentLengthRange: fmt.Sprintf("0,%d", fileSize),
		AllowFileType:      strings.Join(handler.Policy.OptionsSerialized.FileType, ","),
	}

	// 生成上傳憑證
	return handler.getUploadCredential(ctx, putPolicy)
}

func (handler Driver) getUploadCredential(ctx context.Context, policy UploadPolicy) (serializer.UploadCredential, error) {
	// 生成上傳策略
	policyJSON, err := json.Marshal(policy)
	if err != nil {
		return serializer.UploadCredential{}, err
	}
	policyEncoded := base64.StdEncoding.EncodeToString(policyJSON)

	// 生成簽名
	elements := []string{"POST", "/" + handler.Policy.BucketName, policyEncoded}
	signStr := handler.Sign(ctx, elements)

	return serializer.UploadCredential{
		Policy: policyEncoded,
		Token:  signStr,
	}, nil
}

// Sign 計算又拍雲的簽名頭
func (handler Driver) Sign(ctx context.Context, elements []string) string {
	password := fmt.Sprintf("%x", md5.Sum([]byte(handler.Policy.SecretKey)))
	mac := hmac.New(sha1.New, []byte(password))
	value := strings.Join(elements, "&")
	mac.Write([]byte(value))
	signStr := base64.StdEncoding.EncodeToString((mac.Sum(nil)))
	return fmt.Sprintf("UPYUN %s:%s", handler.Policy.AccessKey, signStr)
}
