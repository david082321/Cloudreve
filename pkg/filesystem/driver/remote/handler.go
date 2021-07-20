package remote

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	model "github.com/cloudreve/Cloudreve/v3/models"
	"github.com/cloudreve/Cloudreve/v3/pkg/auth"
	"github.com/cloudreve/Cloudreve/v3/pkg/filesystem/fsctx"
	"github.com/cloudreve/Cloudreve/v3/pkg/filesystem/response"
	"github.com/cloudreve/Cloudreve/v3/pkg/request"
	"github.com/cloudreve/Cloudreve/v3/pkg/serializer"
)

// Driver 遠端儲存策略適配器
type Driver struct {
	Client       request.Client
	Policy       *model.Policy
	AuthInstance auth.Auth
}

// List 列取文件
func (handler Driver) List(ctx context.Context, path string, recursive bool) ([]response.Object, error) {
	var res []response.Object

	reqBody := serializer.ListRequest{
		Path:      path,
		Recursive: recursive,
	}
	reqBodyEncoded, err := json.Marshal(reqBody)
	if err != nil {
		return res, err
	}

	// 發送列表請求
	bodyReader := strings.NewReader(string(reqBodyEncoded))
	signTTL := model.GetIntSetting("slave_api_timeout", 60)
	resp, err := handler.Client.Request(
		"POST",
		handler.getAPIUrl("list"),
		bodyReader,
		request.WithCredential(handler.AuthInstance, int64(signTTL)),
	).CheckHTTPResponse(200).DecodeResponse()
	if err != nil {
		return res, err
	}

	// 處理列取結果
	if resp.Code != 0 {
		return res, errors.New(resp.Error)
	}

	if resStr, ok := resp.Data.(string); ok {
		err = json.Unmarshal([]byte(resStr), &res)
		if err != nil {
			return res, err
		}
	}

	return res, nil
}

// getAPIUrl 獲取介面請求地址
func (handler Driver) getAPIUrl(scope string, routes ...string) string {
	serverURL, err := url.Parse(handler.Policy.Server)
	if err != nil {
		return ""
	}
	var controller *url.URL

	switch scope {
	case "delete":
		controller, _ = url.Parse("/api/v3/slave/delete")
	case "thumb":
		controller, _ = url.Parse("/api/v3/slave/thumb")
	case "list":
		controller, _ = url.Parse("/api/v3/slave/list")
	default:
		controller = serverURL
	}

	for _, r := range routes {
		controller.Path = path.Join(controller.Path, r)
	}

	return serverURL.ResolveReference(controller).String()
}

// Get 獲取文件內容
func (handler Driver) Get(ctx context.Context, path string) (response.RSCloser, error) {
	// 嘗試獲取速度限制 TODO 是否需要在這裡限制？
	speedLimit := 0
	if user, ok := ctx.Value(fsctx.UserCtx).(model.User); ok {
		speedLimit = user.Group.SpeedLimit
	}

	// 獲取文件源地址
	downloadURL, err := handler.Source(ctx, path, url.URL{}, 0, true, speedLimit)
	if err != nil {
		return nil, err
	}

	// 獲取文件資料流
	resp, err := handler.Client.Request(
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

	// 嘗試獲取檔案大小
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
	policy := serializer.UploadPolicy{
		SavePath:   path.Dir(dst),
		FileName:   path.Base(dst),
		AutoRename: false,
		MaxSize:    size,
	}
	credential, err := handler.getUploadCredential(ctx, policy, int64(credentialTTL))
	if err != nil {
		return err
	}

	// 對檔案名進行URLEncode
	fileName, err := url.QueryUnescape(path.Base(dst))
	if err != nil {
		return err
	}

	// 決定是否要禁用文件覆蓋
	overwrite := "true"
	if ctx.Value(fsctx.DisableOverwrite) != nil {
		overwrite = "false"
	}

	// 上傳文件
	resp, err := handler.Client.Request(
		"POST",
		handler.Policy.GetUploadURL(),
		file,
		request.WithHeader(map[string][]string{
			"Authorization": {credential.Token},
			"X-Policy":      {credential.Policy},
			"X-FileName":    {fileName},
			"X-Overwrite":   {overwrite},
		}),
		request.WithContentLength(int64(size)),
		request.WithTimeout(time.Duration(0)),
	).CheckHTTPResponse(200).DecodeResponse()
	if err != nil {
		return err
	}
	if resp.Code != 0 {
		return errors.New(resp.Msg)
	}

	return nil
}

// Delete 刪除一個或多個文件，
// 返回未刪除的文件，及遇到的最後一個錯誤
func (handler Driver) Delete(ctx context.Context, files []string) ([]string, error) {
	// 封裝介面請求正文
	reqBody := serializer.RemoteDeleteRequest{
		Files: files,
	}
	reqBodyEncoded, err := json.Marshal(reqBody)
	if err != nil {
		return files, err
	}

	// 發送刪除請求
	bodyReader := strings.NewReader(string(reqBodyEncoded))
	signTTL := model.GetIntSetting("slave_api_timeout", 60)
	resp, err := handler.Client.Request(
		"POST",
		handler.getAPIUrl("delete"),
		bodyReader,
		request.WithCredential(handler.AuthInstance, int64(signTTL)),
	).CheckHTTPResponse(200).GetResponse()
	if err != nil {
		return files, err
	}

	// 處理刪除結果
	var reqResp serializer.Response
	err = json.Unmarshal([]byte(resp), &reqResp)
	if err != nil {
		return files, err
	}
	if reqResp.Code != 0 {
		var failedResp serializer.RemoteDeleteRequest
		if failed, ok := reqResp.Data.(string); ok {
			err = json.Unmarshal([]byte(failed), &failedResp)
			if err == nil {
				return failedResp.Files, errors.New(reqResp.Error)
			}
		}
		return files, errors.New("未知的返回結果格式")
	}

	return []string{}, nil
}

// Thumb 獲取文件縮圖
func (handler Driver) Thumb(ctx context.Context, path string) (*response.ContentResponse, error) {
	sourcePath := base64.RawURLEncoding.EncodeToString([]byte(path))
	thumbURL := handler.getAPIUrl("thumb") + "/" + sourcePath
	ttl := model.GetIntSetting("preview_timeout", 60)
	signedThumbURL, err := auth.SignURI(handler.AuthInstance, thumbURL, int64(ttl))
	if err != nil {
		return nil, err
	}

	return &response.ContentResponse{
		Redirect: true,
		URL:      signedThumbURL.String(),
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
	fileName := "file"
	if file, ok := ctx.Value(fsctx.FileModelCtx).(model.File); ok {
		fileName = file.Name
	}

	serverURL, err := url.Parse(handler.Policy.Server)
	if err != nil {
		return "", errors.New("無法解析遠端服務端地址")
	}

	// 是否啟用了CDN
	if handler.Policy.BaseURL != "" {
		cdnURL, err := url.Parse(handler.Policy.BaseURL)
		if err != nil {
			return "", err
		}
		serverURL = cdnURL
	}

	var (
		signedURI  *url.URL
		controller = "/api/v3/slave/download"
	)
	if !isDownload {
		controller = "/api/v3/slave/source"
	}

	// 簽名下載網址
	sourcePath := base64.RawURLEncoding.EncodeToString([]byte(path))
	signedURI, err = auth.SignURI(
		handler.AuthInstance,
		fmt.Sprintf("%s/%d/%s/%s", controller, speed, sourcePath, fileName),
		ttl,
	)

	if err != nil {
		return "", serializer.NewError(serializer.CodeEncryptError, "無法對URL進行簽名", err)
	}

	finalURL := serverURL.ResolveReference(signedURI).String()
	return finalURL, nil

}

// Token 獲取上傳策略和認證Token
func (handler Driver) Token(ctx context.Context, TTL int64, key string) (serializer.UploadCredential, error) {
	// 生成回調地址
	siteURL := model.GetSiteURL()
	apiBaseURI, _ := url.Parse("/api/v3/callback/remote/" + key)
	apiURL := siteURL.ResolveReference(apiBaseURI)

	// 生成上傳策略
	policy := serializer.UploadPolicy{
		SavePath:         handler.Policy.DirNameRule,
		FileName:         handler.Policy.FileNameRule,
		AutoRename:       handler.Policy.AutoRename,
		MaxSize:          handler.Policy.MaxSize,
		AllowedExtension: handler.Policy.OptionsSerialized.FileType,
		CallbackURL:      apiURL.String(),
	}
	return handler.getUploadCredential(ctx, policy, TTL)
}

func (handler Driver) getUploadCredential(ctx context.Context, policy serializer.UploadPolicy, TTL int64) (serializer.UploadCredential, error) {
	policyEncoded, err := policy.EncodeUploadPolicy()
	if err != nil {
		return serializer.UploadCredential{}, err
	}

	// 簽名上傳策略
	uploadRequest, _ := http.NewRequest("POST", "/api/v3/slave/upload", nil)
	uploadRequest.Header = map[string][]string{
		"X-Policy":    {policyEncoded},
		"X-Overwrite": {"false"},
	}
	auth.SignRequest(handler.AuthInstance, uploadRequest, TTL)

	if credential, ok := uploadRequest.Header["Authorization"]; ok && len(credential) == 1 {
		return serializer.UploadCredential{
			Token:  credential[0],
			Policy: policyEncoded,
		}, nil
	}
	return serializer.UploadCredential{}, errors.New("無法簽名上傳策略")
}
