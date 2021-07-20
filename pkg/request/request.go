package request

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"time"

	model "github.com/cloudreve/Cloudreve/v3/models"
	"github.com/cloudreve/Cloudreve/v3/pkg/auth"
	"github.com/cloudreve/Cloudreve/v3/pkg/serializer"
	"github.com/cloudreve/Cloudreve/v3/pkg/util"
)

// GeneralClient 通用 HTTP Client
var GeneralClient Client = HTTPClient{}

// Response 請求的響應或錯誤訊息
type Response struct {
	Err      error
	Response *http.Response
}

// Client 請求用戶端
type Client interface {
	Request(method, target string, body io.Reader, opts ...Option) *Response
}

// HTTPClient 實現 Client 介面
type HTTPClient struct {
}

// Option 發送請求的額外設定
type Option interface {
	apply(*options)
}

type options struct {
	timeout       time.Duration
	header        http.Header
	sign          auth.Auth
	signTTL       int64
	ctx           context.Context
	contentLength int64
}

type optionFunc func(*options)

func (f optionFunc) apply(o *options) {
	f(o)
}

func newDefaultOption() *options {
	return &options{
		header:        http.Header{},
		timeout:       time.Duration(30) * time.Second,
		contentLength: -1,
	}
}

// WithTimeout 設定請求超時
func WithTimeout(t time.Duration) Option {
	return optionFunc(func(o *options) {
		o.timeout = t
	})
}

// WithContext 設定請求上下文
func WithContext(c context.Context) Option {
	return optionFunc(func(o *options) {
		o.ctx = c
	})
}

// WithCredential 對請求進行簽名
func WithCredential(instance auth.Auth, ttl int64) Option {
	return optionFunc(func(o *options) {
		o.sign = instance
		o.signTTL = ttl
	})
}

// WithHeader 設定請求Header
func WithHeader(header http.Header) Option {
	return optionFunc(func(o *options) {
		for k, v := range header {
			o.header[k] = v
		}
	})
}

// WithoutHeader 設定清除請求Header
func WithoutHeader(header []string) Option {
	return optionFunc(func(o *options) {
		for _, v := range header {
			delete(o.header, v)
		}

	})
}

// WithContentLength 設定請求大小
func WithContentLength(s int64) Option {
	return optionFunc(func(o *options) {
		o.contentLength = s
	})
}

// Request 發送HTTP請求
func (c HTTPClient) Request(method, target string, body io.Reader, opts ...Option) *Response {
	// 應用額外設定
	options := newDefaultOption()
	for _, o := range opts {
		o.apply(options)
	}

	// 建立請求用戶端
	client := &http.Client{Timeout: options.timeout}

	// size為0時將body設為nil
	if options.contentLength == 0 {
		body = nil
	}

	// 建立請求
	var (
		req *http.Request
		err error
	)
	if options.ctx != nil {
		req, err = http.NewRequestWithContext(options.ctx, method, target, body)
	} else {
		req, err = http.NewRequest(method, target, body)
	}
	if err != nil {
		return &Response{Err: err}
	}

	// 添加請求相關設定
	req.Header = options.header
	if options.contentLength != -1 {
		req.ContentLength = options.contentLength
	}

	// 簽名請求
	if options.sign != nil {
		auth.SignRequest(options.sign, req, options.signTTL)
	}

	// 發送請求
	resp, err := client.Do(req)
	if err != nil {
		return &Response{Err: err}
	}

	return &Response{Err: nil, Response: resp}
}

// GetResponse 檢查響應並獲取響應正文
func (resp *Response) GetResponse() (string, error) {
	if resp.Err != nil {
		return "", resp.Err
	}
	respBody, err := ioutil.ReadAll(resp.Response.Body)
	_ = resp.Response.Body.Close()

	return string(respBody), err
}

// CheckHTTPResponse 檢查請求響應HTTP狀態碼
func (resp *Response) CheckHTTPResponse(status int) *Response {
	if resp.Err != nil {
		return resp
	}

	// 檢查HTTP狀態碼
	if resp.Response.StatusCode != status {
		resp.Err = fmt.Errorf("伺服器返回非正常HTTP狀態%d", resp.Response.StatusCode)
	}
	return resp
}

// DecodeResponse 嘗試解析為serializer.Response，並對狀態碼進行檢查
func (resp *Response) DecodeResponse() (*serializer.Response, error) {
	if resp.Err != nil {
		return nil, resp.Err
	}

	respString, err := resp.GetResponse()
	if err != nil {
		return nil, err
	}

	var res serializer.Response
	err = json.Unmarshal([]byte(respString), &res)
	if err != nil {
		util.Log().Debug("無法解析回調服務端響應：%s", string(respString))
		return nil, err
	}
	return &res, nil
}

// NopRSCloser 實現不完整seeker
type NopRSCloser struct {
	body   io.ReadCloser
	status *rscStatus
}

type rscStatus struct {
	// http.ServeContent 會讀取一小塊以決定內容類型，
	// 但是響應body無法實現seek，所以此項為真時第一個read會返回假資料
	IgnoreFirst bool

	Size int64
}

// GetRSCloser 返回帶有空seeker的RSCloser，供http.ServeContent使用
func (resp *Response) GetRSCloser() (*NopRSCloser, error) {
	if resp.Err != nil {
		return nil, resp.Err
	}

	return &NopRSCloser{
		body: resp.Response.Body,
		status: &rscStatus{
			Size: resp.Response.ContentLength,
		},
	}, resp.Err
}

// SetFirstFakeChunk 開啟第一次read返回空資料
// TODO 測試
func (instance NopRSCloser) SetFirstFakeChunk() {
	instance.status.IgnoreFirst = true
}

// SetContentLength 設定資料流大小
func (instance NopRSCloser) SetContentLength(size int64) {
	instance.status.Size = size
}

// Read 實現 NopRSCloser reader
func (instance NopRSCloser) Read(p []byte) (n int, err error) {
	if instance.status.IgnoreFirst && len(p) == 512 {
		return 0, io.EOF
	}
	return instance.body.Read(p)
}

// Close 實現 NopRSCloser closer
func (instance NopRSCloser) Close() error {
	return instance.body.Close()
}

// Seek 實現 NopRSCloser seeker, 只實現seek開頭/結尾以便http.ServeContent用於確定正文大小
func (instance NopRSCloser) Seek(offset int64, whence int) (int64, error) {
	// 進行第一次Seek操作後，取消忽略選項
	if instance.status.IgnoreFirst {
		instance.status.IgnoreFirst = false
	}
	if offset == 0 {
		switch whence {
		case io.SeekStart:
			return 0, nil
		case io.SeekEnd:
			return instance.status.Size, nil
		}
	}
	return 0, errors.New("未實現")

}

// BlackHole 將用戶端發來的資料放入黑洞
func BlackHole(r io.Reader) {
	if !model.IsTrueVal(model.GetSettingByName("reset_after_upload_failed")) {
		_, err := io.Copy(ioutil.Discard, r)
		if err != nil {
			util.Log().Debug("黑洞資料出錯，%s", err)
		}
	}
}
