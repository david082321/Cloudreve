package user

import (
	"fmt"
	model "github.com/cloudreve/Cloudreve/v3/models"
	"github.com/cloudreve/Cloudreve/v3/pkg/cache"
	"github.com/cloudreve/Cloudreve/v3/pkg/email"
	"github.com/cloudreve/Cloudreve/v3/pkg/hashid"
	"github.com/cloudreve/Cloudreve/v3/pkg/serializer"
	"github.com/cloudreve/Cloudreve/v3/pkg/util"
	"github.com/gin-gonic/gin"
	"github.com/pquerna/otp/totp"
	"net/url"
)

// UserLoginService 管理使用者登入的服務
type UserLoginService struct {
	//TODO 細緻調整驗證規則
	UserName string `form:"userName" json:"userName" binding:"required,email"`
	Password string `form:"Password" json:"Password" binding:"required,min=4,max=64"`
}

// UserResetEmailService 發送密碼重設郵件服務
type UserResetEmailService struct {
	UserName string `form:"userName" json:"userName" binding:"required,email"`
}

// UserResetService 密碼重設服務
type UserResetService struct {
	Password string `form:"Password" json:"Password" binding:"required,min=4,max=64"`
	ID       string `json:"id" binding:"required"`
	Secret   string `json:"secret" binding:"required"`
}

// Reset 重設密碼
func (service *UserResetService) Reset(c *gin.Context) serializer.Response {
	// 取得原始使用者ID
	uid, err := hashid.DecodeHashID(service.ID, hashid.UserID)
	if err != nil {
		return serializer.Err(serializer.CodeNotFound, "重設連結無效", err)
	}

	// 檢查重設工作階段
	resetSession, exist := cache.Get(fmt.Sprintf("user_reset_%d", uid))
	if !exist || resetSession.(string) != service.Secret {
		return serializer.Err(serializer.CodeNotFound, "連結已過期", err)
	}

	// 重設使用者密碼
	user, err := model.GetActiveUserByID(uid)
	if err != nil {
		return serializer.Err(serializer.CodeNotFound, "使用者不存在", err)
	}

	user.SetPassword(service.Password)
	if err := user.Update(map[string]interface{}{"password": user.Password}); err != nil {
		return serializer.DBErr("無法重設密碼", err)
	}

	cache.Deletes([]string{fmt.Sprintf("%d", uid)}, "user_reset_")
	return serializer.Response{}
}

// Reset 發送密碼重設郵件
func (service *UserResetEmailService) Reset(c *gin.Context) serializer.Response {
	// 尋找使用者
	if user, err := model.GetUserByEmail(service.UserName); err == nil {

		if user.Status == model.Baned || user.Status == model.OveruseBaned {
			return serializer.Err(403, "該帳號已被封禁", nil)
		}
		if user.Status == model.NotActivicated {
			return serializer.Err(403, "該帳號未啟動", nil)
		}
		// 建立密碼重設工作階段
		secret := util.RandStringRunes(32)
		cache.Set(fmt.Sprintf("user_reset_%d", user.ID), secret, 3600)

		// 生成使用者訪問的重設連結
		controller, _ := url.Parse("/reset")
		finalURL := model.GetSiteURL().ResolveReference(controller)
		queries := finalURL.Query()
		queries.Add("id", hashid.HashID(user.ID, hashid.UserID))
		queries.Add("sign", secret)
		finalURL.RawQuery = queries.Encode()

		// 發送密碼重設郵件
		title, body := email.NewResetEmail(user.Nick, finalURL.String())
		if err := email.Send(user.Email, title, body); err != nil {
			return serializer.Err(serializer.CodeInternalSetting, "無法發送密碼重設郵件", err)
		}

	}

	return serializer.Response{}
}

// Login 二步驗證繼續登入
func (service *Enable2FA) Login(c *gin.Context) serializer.Response {
	if uid, ok := util.GetSession(c, "2fa_user_id").(uint); ok {
		// 尋找使用者
		expectedUser, err := model.GetActiveUserByID(uid)
		if err != nil {
			return serializer.Err(serializer.CodeNotFound, "使用者不存在", nil)
		}

		// 驗證二步驗證程式碼
		if !totp.Validate(service.Code, expectedUser.TwoFactor) {
			return serializer.ParamErr("驗證程式碼不正確", nil)
		}

		//登入成功，清空並設定session
		util.DeleteSession(c, "2fa_user_id")
		util.SetSession(c, map[string]interface{}{
			"user_id": expectedUser.ID,
		})

		return serializer.BuildUserResponse(expectedUser)
	}

	return serializer.Err(serializer.CodeNotFound, "登入工作階段不存在", nil)
}

// Login 使用者登入函數
func (service *UserLoginService) Login(c *gin.Context) serializer.Response {
	expectedUser, err := model.GetUserByEmail(service.UserName)
	// 一系列校驗
	if err != nil {
		return serializer.Err(serializer.CodeCredentialInvalid, "使用者信箱或密碼錯誤", err)
	}
	if authOK, _ := expectedUser.CheckPassword(service.Password); !authOK {
		return serializer.Err(serializer.CodeCredentialInvalid, "使用者信箱或密碼錯誤", nil)
	}
	if expectedUser.Status == model.Baned || expectedUser.Status == model.OveruseBaned {
		return serializer.Err(403, "該帳號已被封禁", nil)
	}
	if expectedUser.Status == model.NotActivicated {
		return serializer.Err(403, "該帳號未啟動", nil)
	}

	if expectedUser.TwoFactor != "" {
		// 需要二步驗證
		util.SetSession(c, map[string]interface{}{
			"2fa_user_id": expectedUser.ID,
		})
		return serializer.Response{Code: 203}
	}

	//登入成功，清空並設定session
	util.SetSession(c, map[string]interface{}{
		"user_id": expectedUser.ID,
	})

	return serializer.BuildUserResponse(expectedUser)

}
