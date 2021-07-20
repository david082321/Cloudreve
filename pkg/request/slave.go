package request

import (
	"bytes"
	"encoding/json"
	"errors"
	"time"

	"github.com/cloudreve/Cloudreve/v3/pkg/auth"
	"github.com/cloudreve/Cloudreve/v3/pkg/conf"
	"github.com/cloudreve/Cloudreve/v3/pkg/serializer"
)

// RemoteCallback 發送遠端儲存策略上傳回調請求
func RemoteCallback(url string, body serializer.UploadCallback) error {
	callbackBody, err := json.Marshal(struct {
		Data serializer.UploadCallback `json:"data"`
	}{
		Data: body,
	})
	if err != nil {
		return serializer.NewError(serializer.CodeCallbackError, "無法編碼回調正文", err)
	}

	resp := GeneralClient.Request(
		"POST",
		url,
		bytes.NewReader(callbackBody),
		WithTimeout(time.Duration(conf.SlaveConfig.CallbackTimeout)*time.Second),
		WithCredential(auth.General, int64(conf.SlaveConfig.SignatureTTL)),
	)

	if resp.Err != nil {
		return serializer.NewError(serializer.CodeCallbackError, "無法發起回調請求", resp.Err)
	}

	// 解析回調服務端響應
	resp = resp.CheckHTTPResponse(200)
	if resp.Err != nil {
		return serializer.NewError(serializer.CodeCallbackError, "伺服器返回異常響應", resp.Err)
	}
	response, err := resp.DecodeResponse()
	if err != nil {
		return serializer.NewError(serializer.CodeCallbackError, "無法解析服務端返回的響應", err)
	}
	if response.Code != 0 {
		return serializer.NewError(response.Code, response.Msg, errors.New(response.Error))
	}

	return nil
}
