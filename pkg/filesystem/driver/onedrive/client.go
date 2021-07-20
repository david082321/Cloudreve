package onedrive

import (
	"errors"

	model "github.com/cloudreve/Cloudreve/v3/models"
	"github.com/cloudreve/Cloudreve/v3/pkg/request"
)

var (
	// ErrAuthEndpoint 無法解析授權端點地址
	ErrAuthEndpoint = errors.New("無法解析授權端點地址")
	// ErrInvalidRefreshToken 上傳策略無有效的RefreshToken
	ErrInvalidRefreshToken = errors.New("上傳策略無有效的RefreshToken")
	// ErrDeleteFile 無法刪除文件
	ErrDeleteFile = errors.New("無法刪除文件")
	// ErrClientCanceled 用戶端取消操作
	ErrClientCanceled = errors.New("用戶端取消操作")
)

// Client OneDrive用戶端
type Client struct {
	Endpoints  *Endpoints
	Policy     *model.Policy
	Credential *Credential

	ClientID     string
	ClientSecret string
	Redirect     string

	Request request.Client
}

// Endpoints OneDrive用戶端相關設定
type Endpoints struct {
	OAuthURL       string // OAuth認證的基URL
	OAuthEndpoints *oauthEndpoint
	EndpointURL    string // 介面請求的基URL
	isInChina      bool   // 是否為世紀互聯
	DriverResource string // 要使用的驅動器
}

// NewClient 根據儲存策略獲取新的client
func NewClient(policy *model.Policy) (*Client, error) {
	client := &Client{
		Endpoints: &Endpoints{
			OAuthURL:       policy.BaseURL,
			EndpointURL:    policy.Server,
			DriverResource: policy.OptionsSerialized.OdDriver,
		},
		Credential: &Credential{
			RefreshToken: policy.AccessKey,
		},
		Policy:       policy,
		ClientID:     policy.BucketName,
		ClientSecret: policy.SecretKey,
		Redirect:     policy.OptionsSerialized.OdRedirect,
		Request:      request.HTTPClient{},
	}

	if client.Endpoints.DriverResource == "" {
		client.Endpoints.DriverResource = "me/drive"
	}

	oauthBase := client.getOAuthEndpoint()
	if oauthBase == nil {
		return nil, ErrAuthEndpoint
	}
	client.Endpoints.OAuthEndpoints = oauthBase

	return client, nil
}
