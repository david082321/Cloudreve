package onedrive

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	model "github.com/cloudreve/Cloudreve/v3/models"
	"github.com/cloudreve/Cloudreve/v3/pkg/cache"
	"github.com/cloudreve/Cloudreve/v3/pkg/filesystem/fsctx"
	"github.com/cloudreve/Cloudreve/v3/pkg/request"
	"github.com/cloudreve/Cloudreve/v3/pkg/util"
)

const (
	// SmallFileSize 單文件上傳介面最大尺寸
	SmallFileSize uint64 = 4 * 1024 * 1024
	// ChunkSize 服務端中轉分片上傳分片大小
	ChunkSize uint64 = 10 * 1024 * 1024
	// ListRetry 列取請求重試次數
	ListRetry = 1
)

// GetSourcePath 獲取文件的絕對路徑
func (info *FileInfo) GetSourcePath() string {
	res, err := url.PathUnescape(
		strings.TrimPrefix(
			path.Join(
				strings.TrimPrefix(info.ParentReference.Path, "/drive/root:"),
				info.Name,
			),
			"/",
		),
	)
	if err != nil {
		return ""
	}
	return res
}

// Error 實現error介面
func (err RespError) Error() string {
	return err.APIError.Message
}

func (client *Client) getRequestURL(api string, opts ...Option) string {
	options := newDefaultOption()
	for _, o := range opts {
		o.apply(options)
	}

	base, _ := url.Parse(client.Endpoints.EndpointURL)
	if base == nil {
		return ""
	}

	if options.useDriverResource {
		base.Path = path.Join(base.Path, client.Endpoints.DriverResource, api)
	} else {
		base.Path = path.Join(base.Path, api)
	}

	return base.String()
}

// ListChildren 根據路徑列取子物件
func (client *Client) ListChildren(ctx context.Context, path string) ([]FileInfo, error) {
	var requestURL string
	dst := strings.TrimPrefix(path, "/")
	if dst == "" {
		requestURL = client.getRequestURL("root/children")
	} else {
		requestURL = client.getRequestURL("root:/" + dst + ":/children")
	}

	res, err := client.requestWithStr(ctx, "GET", requestURL+"?$top=999999999", "", 200)
	if err != nil {
		retried := 0
		if v, ok := ctx.Value(fsctx.RetryCtx).(int); ok {
			retried = v
		}
		if retried < ListRetry {
			retried++
			util.Log().Debug("路徑[%s]列取請求失敗[%s]，5秒鐘後重試", path, err)
			time.Sleep(time.Duration(5) * time.Second)
			return client.ListChildren(context.WithValue(ctx, fsctx.RetryCtx, retried), path)
		}
		return nil, err
	}

	var (
		decodeErr error
		fileInfo  ListResponse
	)
	decodeErr = json.Unmarshal([]byte(res), &fileInfo)
	if decodeErr != nil {
		return nil, decodeErr
	}

	return fileInfo.Value, nil
}

// Meta 根據資源ID或文件路徑獲取文件元訊息
func (client *Client) Meta(ctx context.Context, id string, path string) (*FileInfo, error) {
	var requestURL string
	if id != "" {
		requestURL = client.getRequestURL("items/" + id)
	} else {
		dst := strings.TrimPrefix(path, "/")
		requestURL = client.getRequestURL("root:/" + dst)
	}

	res, err := client.requestWithStr(ctx, "GET", requestURL+"?expand=thumbnails", "", 200)
	if err != nil {
		return nil, err
	}

	var (
		decodeErr error
		fileInfo  FileInfo
	)
	decodeErr = json.Unmarshal([]byte(res), &fileInfo)
	if decodeErr != nil {
		return nil, decodeErr
	}

	return &fileInfo, nil

}

// CreateUploadSession 建立分片上傳工作階段
func (client *Client) CreateUploadSession(ctx context.Context, dst string, opts ...Option) (string, error) {
	options := newDefaultOption()
	for _, o := range opts {
		o.apply(options)
	}

	dst = strings.TrimPrefix(dst, "/")
	requestURL := client.getRequestURL("root:/" + dst + ":/createUploadSession")
	body := map[string]map[string]interface{}{
		"item": {
			"@microsoft.graph.conflictBehavior": options.conflictBehavior,
		},
	}
	bodyBytes, _ := json.Marshal(body)

	res, err := client.requestWithStr(ctx, "POST", requestURL, string(bodyBytes), 200)
	if err != nil {
		return "", err
	}

	var (
		decodeErr     error
		uploadSession UploadSessionResponse
	)
	decodeErr = json.Unmarshal([]byte(res), &uploadSession)
	if decodeErr != nil {
		return "", decodeErr
	}

	return uploadSession.UploadURL, nil
}

// GetSiteIDByURL 通過 SharePoint 站點 URL 獲取站點ID
func (client *Client) GetSiteIDByURL(ctx context.Context, siteUrl string) (string, error) {
	siteUrlParsed, err := url.Parse(siteUrl)
	if err != nil {
		return "", err
	}

	hostName := siteUrlParsed.Hostname()
	relativePath := strings.Trim(siteUrlParsed.Path, "/")
	requestURL := client.getRequestURL(fmt.Sprintf("sites/%s:/%s", hostName, relativePath), WithDriverResource(false))
	res, reqErr := client.requestWithStr(ctx, "GET", requestURL, "", 200)
	if reqErr != nil {
		return "", reqErr
	}

	var (
		decodeErr error
		siteInfo  Site
	)
	decodeErr = json.Unmarshal([]byte(res), &siteInfo)
	if decodeErr != nil {
		return "", decodeErr
	}

	return siteInfo.ID, nil
}

// GetUploadSessionStatus 查詢上傳工作階段狀態
func (client *Client) GetUploadSessionStatus(ctx context.Context, uploadURL string) (*UploadSessionResponse, error) {
	res, err := client.requestWithStr(ctx, "GET", uploadURL, "", 200)
	if err != nil {
		return nil, err
	}

	var (
		decodeErr     error
		uploadSession UploadSessionResponse
	)
	decodeErr = json.Unmarshal([]byte(res), &uploadSession)
	if decodeErr != nil {
		return nil, decodeErr
	}

	return &uploadSession, nil
}

// UploadChunk 上傳分片
func (client *Client) UploadChunk(ctx context.Context, uploadURL string, chunk *Chunk) (*UploadSessionResponse, error) {
	res, err := client.request(
		ctx, "PUT", uploadURL, bytes.NewReader(chunk.Data[0:chunk.ChunkSize]),
		request.WithContentLength(int64(chunk.ChunkSize)),
		request.WithHeader(http.Header{
			"Content-Range": {fmt.Sprintf("bytes %d-%d/%d", chunk.Offset, chunk.Offset+chunk.ChunkSize-1, chunk.Total)},
		}),
		request.WithoutHeader([]string{"Authorization", "Content-Type"}),
		request.WithTimeout(time.Duration(300)*time.Second),
	)
	if err != nil {
		// 如果重試次數小於限制，5秒後重試
		if chunk.Retried < model.GetIntSetting("onedrive_chunk_retries", 1) {
			chunk.Retried++
			util.Log().Debug("分片偏移%d上傳失敗[%s]，5秒鐘後重試", chunk.Offset, err)
			time.Sleep(time.Duration(5) * time.Second)
			return client.UploadChunk(ctx, uploadURL, chunk)
		}
		return nil, err
	}

	if chunk.IsLast() {
		return nil, nil
	}

	var (
		decodeErr error
		uploadRes UploadSessionResponse
	)
	decodeErr = json.Unmarshal([]byte(res), &uploadRes)
	if decodeErr != nil {
		return nil, decodeErr
	}

	return &uploadRes, nil
}

// Upload 上傳文件
func (client *Client) Upload(ctx context.Context, dst string, size int, file io.Reader) error {
	// 決定是否覆蓋文件
	overwrite := "replace"
	if ctx.Value(fsctx.DisableOverwrite) != nil {
		overwrite = "fail"
	}

	// 小文件，使用簡單上傳介面上傳
	if size <= int(SmallFileSize) {
		_, err := client.SimpleUpload(ctx, dst, file, int64(size), WithConflictBehavior(overwrite))
		return err
	}

	// 大文件，進行分片
	// 建立上傳工作階段
	uploadURL, err := client.CreateUploadSession(ctx, dst, WithConflictBehavior(overwrite))
	if err != nil {
		return err
	}

	offset := 0
	chunkNum := size / int(ChunkSize)
	if size%int(ChunkSize) != 0 {
		chunkNum++
	}

	chunkData := make([]byte, ChunkSize)

	for i := 0; i < chunkNum; i++ {
		select {
		case <-ctx.Done():
			util.Log().Debug("OneDrive 用戶端取消")
			return ErrClientCanceled
		default:
			// 分塊
			chunkSize := int(ChunkSize)
			if size-offset < chunkSize {
				chunkSize = size - offset
			}

			// 因為後面需要錯誤重試，這裡要把分片內容讀到記憶體中
			chunkContent := chunkData[:chunkSize]
			_, err := io.ReadFull(file, chunkContent)

			chunk := Chunk{
				Offset:    offset,
				ChunkSize: chunkSize,
				Total:     size,
				Data:      chunkContent,
			}

			// 上傳
			_, err = client.UploadChunk(ctx, uploadURL, &chunk)
			if err != nil {
				return err
			}
			offset += chunkSize
		}

	}
	return nil
}

// DeleteUploadSession 刪除上傳工作階段
func (client *Client) DeleteUploadSession(ctx context.Context, uploadURL string) error {
	_, err := client.requestWithStr(ctx, "DELETE", uploadURL, "", 204)
	if err != nil {
		return err
	}

	return nil
}

// SimpleUpload 上傳小文件到dst
func (client *Client) SimpleUpload(ctx context.Context, dst string, body io.Reader, size int64, opts ...Option) (*UploadResult, error) {
	options := newDefaultOption()
	for _, o := range opts {
		o.apply(options)
	}

	dst = strings.TrimPrefix(dst, "/")
	requestURL := client.getRequestURL("root:/" + dst + ":/content")
	requestURL += ("?@microsoft.graph.conflictBehavior=" + options.conflictBehavior)

	res, err := client.request(ctx, "PUT", requestURL, body, request.WithContentLength(int64(size)),
		request.WithTimeout(time.Duration(150)*time.Second),
	)
	if err != nil {
		retried := 0
		if v, ok := ctx.Value(fsctx.RetryCtx).(int); ok {
			retried = v
		}
		if retried < model.GetIntSetting("onedrive_chunk_retries", 1) {
			retried++
			util.Log().Debug("文件[%s]上傳失敗[%s]，5秒鐘後重試", dst, err)
			time.Sleep(time.Duration(5) * time.Second)
			return client.SimpleUpload(context.WithValue(ctx, fsctx.RetryCtx, retried), dst, body, size, opts...)
		}
		return nil, err
	}

	var (
		decodeErr error
		uploadRes UploadResult
	)
	decodeErr = json.Unmarshal([]byte(res), &uploadRes)
	if decodeErr != nil {
		return nil, decodeErr
	}

	return &uploadRes, nil
}

// BatchDelete 並行刪除給出的文件，返回刪除失敗的文件，及第一個遇到的錯誤。此方法將文件分為
// 20個一組，呼叫Delete並行刪除
// TODO 測試
func (client *Client) BatchDelete(ctx context.Context, dst []string) ([]string, error) {
	groupNum := len(dst)/20 + 1
	finalRes := make([]string, 0, len(dst))
	res := make([]string, 0, 20)
	var err error

	for i := 0; i < groupNum; i++ {
		end := 20*i + 20
		if i == groupNum-1 {
			end = len(dst)
		}
		res, err = client.Delete(ctx, dst[20*i:end])
		finalRes = append(finalRes, res...)
	}

	return finalRes, err
}

// Delete 並行刪除文件，返回刪除失敗的文件，及第一個遇到的錯誤，
// 由於API限制，最多刪除20個
func (client *Client) Delete(ctx context.Context, dst []string) ([]string, error) {
	body := client.makeBatchDeleteRequestsBody(dst)
	res, err := client.requestWithStr(ctx, "POST", client.getRequestURL("$batch",
		WithDriverResource(false)), body, 200)
	if err != nil {
		return dst, err
	}

	var (
		decodeErr error
		deleteRes BatchResponses
	)
	decodeErr = json.Unmarshal([]byte(res), &deleteRes)
	if decodeErr != nil {
		return dst, decodeErr
	}

	// 取得刪除失敗的文件
	failed := getDeleteFailed(&deleteRes)
	if len(failed) != 0 {
		return failed, ErrDeleteFile
	}
	return failed, nil
}

func getDeleteFailed(res *BatchResponses) []string {
	var failed = make([]string, 0, len(res.Responses))
	for _, v := range res.Responses {
		if v.Status != 204 && v.Status != 404 {
			failed = append(failed, v.ID)
		}
	}
	return failed
}

// makeBatchDeleteRequestsBody 生成批次刪除請求正文
func (client *Client) makeBatchDeleteRequestsBody(files []string) string {
	req := BatchRequests{
		Requests: make([]BatchRequest, len(files)),
	}
	for i, v := range files {
		v = strings.TrimPrefix(v, "/")
		filePath, _ := url.Parse("/" + client.Endpoints.DriverResource + "/root:/")
		filePath.Path = path.Join(filePath.Path, v)
		req.Requests[i] = BatchRequest{
			ID:     v,
			Method: "DELETE",
			URL:    filePath.EscapedPath(),
		}
	}

	res, _ := json.Marshal(req)
	return string(res)
}

// GetThumbURL 獲取給定尺寸的縮圖URL
func (client *Client) GetThumbURL(ctx context.Context, dst string, w, h uint) (string, error) {
	dst = strings.TrimPrefix(dst, "/")
	requestURL := client.getRequestURL("root:/"+dst+":/thumbnails/0") + "/large"

	res, err := client.requestWithStr(ctx, "GET", requestURL, "", 200)
	if err != nil {
		return "", err
	}

	var (
		decodeErr error
		thumbRes  ThumbResponse
	)
	decodeErr = json.Unmarshal([]byte(res), &thumbRes)
	if decodeErr != nil {
		return "", decodeErr
	}

	if thumbRes.URL != "" {
		return thumbRes.URL, nil
	}

	if len(thumbRes.Value) == 1 {
		if res, ok := thumbRes.Value[0]["large"]; ok {
			return res.(map[string]interface{})["url"].(string), nil
		}
	}

	return "", errors.New("無法生成縮圖")
}

// MonitorUpload 監控用戶端分片上傳進度
func (client *Client) MonitorUpload(uploadURL, callbackKey, path string, size uint64, ttl int64) {
	// 回調完成通知chan
	callbackChan := make(chan bool)
	callbackSignal.Store(callbackKey, callbackChan)
	defer callbackSignal.Delete(callbackKey)
	timeout := model.GetIntSetting("onedrive_monitor_timeout", 600)
	interval := model.GetIntSetting("onedrive_callback_check", 20)

	for {
		select {
		case <-callbackChan:
			util.Log().Debug("用戶端完成回調")
			return
		case <-time.After(time.Duration(ttl) * time.Second):
			// 上傳工作階段到期，仍未完成上傳，建立占位符
			client.DeleteUploadSession(context.Background(), uploadURL)
			_, err := client.SimpleUpload(context.Background(), path, strings.NewReader(""), 0, WithConflictBehavior("replace"))
			if err != nil {
				util.Log().Debug("無法建立占位文件，%s", err)
			}
			return
		case <-time.After(time.Duration(timeout) * time.Second):
			util.Log().Debug("檢查上傳情況")
			status, err := client.GetUploadSessionStatus(context.Background(), uploadURL)

			if err != nil {
				if resErr, ok := err.(*RespError); ok {
					if resErr.APIError.Code == "itemNotFound" {
						util.Log().Debug("上傳工作階段已完成，稍後檢查回調")
						time.Sleep(time.Duration(interval) * time.Second)
						util.Log().Debug("開始檢查回調")
						_, ok := cache.Get("callback_" + callbackKey)
						if ok {
							util.Log().Warning("未發送回調，刪除文件")
							cache.Deletes([]string{callbackKey}, "callback_")
							_, err = client.Delete(context.Background(), []string{path})
							if err != nil {
								util.Log().Warning("無法刪除未回調的文件，%s", err)
							}
						}
						return
					}
				}
				util.Log().Debug("無法獲取上傳工作階段狀態，繼續下一輪，%s", err.Error())
				continue
			}

			// 成功獲取分片上傳狀態，檢查檔案大小
			if len(status.NextExpectedRanges) == 0 {
				continue
			}
			sizeRange := strings.Split(
				status.NextExpectedRanges[len(status.NextExpectedRanges)-1],
				"-",
			)
			if len(sizeRange) != 2 {
				continue
			}
			uploadFullSize, _ := strconv.ParseUint(sizeRange[1], 10, 64)
			if (sizeRange[0] == "0" && sizeRange[1] == "") || uploadFullSize+1 != size {
				util.Log().Debug("未開始上傳或檔案大小不一致，取消上傳工作階段")
				// 取消上傳工作階段，實測OneDrive取消上傳工作階段後，用戶端還是可以上傳，
				// 所以上傳一個空文件占位，阻止用戶端上傳
				client.DeleteUploadSession(context.Background(), uploadURL)
				_, err := client.SimpleUpload(context.Background(), path, strings.NewReader(""), 0, WithConflictBehavior("replace"))
				if err != nil {
					util.Log().Debug("無法建立占位文件，%s", err)
				}
				return
			}

		}
	}
}

// FinishCallback 向Monitor發送回調結束訊號
func FinishCallback(key string) {
	if signal, ok := callbackSignal.Load(key); ok {
		if signalChan, ok := signal.(chan bool); ok {
			close(signalChan)
		}
	}
}

func sysError(err error) *RespError {
	return &RespError{APIError: APIError{
		Code:    "system",
		Message: err.Error(),
	}}
}

func (client *Client) request(ctx context.Context, method string, url string, body io.Reader, option ...request.Option) (string, *RespError) {
	// 獲取憑證
	err := client.UpdateCredential(ctx)
	if err != nil {
		return "", sysError(err)
	}

	option = append(option,
		request.WithHeader(http.Header{
			"Authorization": {"Bearer " + client.Credential.AccessToken},
			"Content-Type":  {"application/json"},
		}),
		request.WithContext(ctx),
	)

	// 發送請求
	res := client.Request.Request(
		method,
		url,
		body,
		option...,
	)

	if res.Err != nil {
		return "", sysError(res.Err)
	}

	respBody, err := res.GetResponse()
	if err != nil {
		return "", sysError(err)
	}

	// 解析請求響應
	var (
		errResp   RespError
		decodeErr error
	)
	// 如果有錯誤
	if res.Response.StatusCode < 200 || res.Response.StatusCode >= 300 {
		decodeErr = json.Unmarshal([]byte(respBody), &errResp)
		if decodeErr != nil {
			util.Log().Debug("Onedrive返回未知響應[%s]", respBody)
			return "", sysError(decodeErr)
		}
		return "", &errResp
	}

	return respBody, nil
}

func (client *Client) requestWithStr(ctx context.Context, method string, url string, body string, expectedCode int) (string, *RespError) {
	// 發送請求
	bodyReader := ioutil.NopCloser(strings.NewReader(body))
	return client.request(ctx, method, url, bodyReader,
		request.WithContentLength(int64(len(body))),
	)
}
