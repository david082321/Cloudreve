package callback

import (
	"context"
	"fmt"

	model "github.com/cloudreve/Cloudreve/v3/models"
	"github.com/cloudreve/Cloudreve/v3/pkg/cache"
	"github.com/cloudreve/Cloudreve/v3/pkg/filesystem/driver/onedrive"
	"github.com/cloudreve/Cloudreve/v3/pkg/serializer"
	"github.com/cloudreve/Cloudreve/v3/pkg/util"
	"github.com/gin-gonic/gin"
	"strings"
)

// OneDriveOauthService OneDrive 授權回調服務
type OneDriveOauthService struct {
	Code     string `form:"code"`
	Error    string `form:"error"`
	ErrorMsg string `form:"error_description"`
}

// Auth 更新認證訊息
func (service *OneDriveOauthService) Auth(c *gin.Context) serializer.Response {
	if service.Error != "" {
		return serializer.ParamErr(service.ErrorMsg, nil)
	}

	policyID, ok := util.GetSession(c, "onedrive_oauth_policy").(uint)
	if !ok {
		return serializer.Err(serializer.CodeNotFound, "授權工作階段不存在，請重試", nil)
	}

	util.DeleteSession(c, "onedrive_oauth_policy")

	policy, err := model.GetPolicyByID(policyID)
	if err != nil {
		return serializer.Err(serializer.CodeNotFound, "儲存策略不存在", nil)
	}

	client, err := onedrive.NewClient(&policy)
	if err != nil {
		return serializer.Err(serializer.CodeInternalSetting, "無法初始化 OneDrive 用戶端", err)
	}

	credential, err := client.ObtainToken(c, onedrive.WithCode(service.Code))
	if err != nil {
		return serializer.Err(serializer.CodeInternalSetting, "AccessToken 獲取失敗", err)
	}

	// 更新儲存策略的 RefreshToken
	client.Policy.AccessKey = credential.RefreshToken
	if err := client.Policy.SaveAndClearCache(); err != nil {
		return serializer.DBErr("無法更新 RefreshToken", err)
	}

	cache.Deletes([]string{client.Policy.AccessKey}, "onedrive_")
	if client.Policy.OptionsSerialized.OdDriver != "" && strings.Contains(client.Policy.OptionsSerialized.OdDriver, "http") {
		if err := querySharePointSiteID(c, client.Policy); err != nil {
			return serializer.Err(serializer.CodeInternalSetting, "無法查詢 SharePoint 站點 ID", err)
		}
	}

	return serializer.Response{}
}

func querySharePointSiteID(ctx context.Context, policy *model.Policy) error {
	client, err := onedrive.NewClient(policy)
	if err != nil {
		return err
	}

	id, err := client.GetSiteIDByURL(ctx, client.Policy.OptionsSerialized.OdDriver)
	if err != nil {
		return err
	}

	client.Policy.OptionsSerialized.OdDriver = fmt.Sprintf("sites/%s/drive", id)
	if err := client.Policy.SaveAndClearCache(); err != nil {
		return err
	}

	return nil
}
