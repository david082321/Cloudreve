package onedrive

import (
	"encoding/gob"
	"net/url"
	"sync"
)

// RespError 介面返回錯誤
type RespError struct {
	APIError APIError `json:"error"`
}

// APIError 介面返回的錯誤內容
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// UploadSessionResponse 分片上傳工作階段
type UploadSessionResponse struct {
	DataContext        string   `json:"@odata.context"`
	ExpirationDateTime string   `json:"expirationDateTime"`
	NextExpectedRanges []string `json:"nextExpectedRanges"`
	UploadURL          string   `json:"uploadUrl"`
}

// FileInfo 文件元訊息
type FileInfo struct {
	Name            string          `json:"name"`
	Size            uint64          `json:"size"`
	Image           imageInfo       `json:"image"`
	ParentReference parentReference `json:"parentReference"`
	DownloadURL     string          `json:"@microsoft.graph.downloadUrl"`
	File            *file           `json:"file"`
	Folder          *folder         `json:"folder"`
}

type file struct {
	MimeType string `json:"mimeType"`
}

type folder struct {
	ChildCount int `json:"childCount"`
}

type imageInfo struct {
	Height int `json:"height"`
	Width  int `json:"width"`
}

type parentReference struct {
	Path string `json:"path"`
	Name string `json:"name"`
	ID   string `json:"id"`
}

// UploadResult 上傳結果
type UploadResult struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Size uint64 `json:"size"`
}

// BatchRequests 批次操作請求
type BatchRequests struct {
	Requests []BatchRequest `json:"requests"`
}

// BatchRequest 批次操作單個請求
type BatchRequest struct {
	ID      string            `json:"id"`
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Body    interface{}       `json:"body,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
}

// BatchResponses 批次操作響應
type BatchResponses struct {
	Responses []BatchResponse `json:"responses"`
}

// BatchResponse 批次操作單個響應
type BatchResponse struct {
	ID     string `json:"id"`
	Status int    `json:"status"`
}

// ThumbResponse 獲取縮圖的響應
type ThumbResponse struct {
	Value []map[string]interface{} `json:"value"`
	URL   string                   `json:"url"`
}

// ListResponse 列取子項目響應
type ListResponse struct {
	Value   []FileInfo `json:"value"`
	Context string     `json:"@odata.context"`
}

// Chunk 文件分片
type Chunk struct {
	Offset    int
	ChunkSize int
	Total     int
	Retried   int
	Data      []byte
}

// oauthEndpoint OAuth介面地址
type oauthEndpoint struct {
	token     url.URL
	authorize url.URL
}

// Credential 獲取token時返回的憑證
type Credential struct {
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	Scope        string `json:"scope"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	UserID       string `json:"user_id"`
}

// OAuthError OAuth相關介面的錯誤響應
type OAuthError struct {
	ErrorType        string `json:"error"`
	ErrorDescription string `json:"error_description"`
	CorrelationID    string `json:"correlation_id"`
}

// Site SharePoint 站點訊息
type Site struct {
	Description string `json:"description"`
	ID          string `json:"id"`
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	WebUrl      string `json:"webUrl"`
}

func init() {
	gob.Register(Credential{})
}

// IsLast 返回是否為最後一個分片
func (chunk *Chunk) IsLast() bool {
	return chunk.Total-chunk.Offset == chunk.ChunkSize
}

var callbackSignal sync.Map
