package controllers

import (
	"encoding/json"
	"fmt"

	model "github.com/cloudreve/Cloudreve/v3/models"
	"github.com/cloudreve/Cloudreve/v3/pkg/authn"
	"github.com/cloudreve/Cloudreve/v3/pkg/request"
	"github.com/cloudreve/Cloudreve/v3/pkg/serializer"
	"github.com/cloudreve/Cloudreve/v3/pkg/thumb"
	"github.com/cloudreve/Cloudreve/v3/pkg/util"
	"github.com/cloudreve/Cloudreve/v3/service/user"
	"github.com/duo-labs/webauthn/webauthn"
	"github.com/gin-gonic/gin"
)

// StartLoginAuthn 開始註冊WebAuthn登入
func StartLoginAuthn(c *gin.Context) {
	userName := c.Param("username")
	expectedUser, err := model.GetActiveUserByEmail(userName)
	if err != nil {
		c.JSON(200, serializer.Err(serializer.CodeNotFound, "使用者不存在", err))
		return
	}

	instance, err := authn.NewAuthnInstance()
	if err != nil {
		c.JSON(200, serializer.Err(serializer.CodeInternalSetting, "無法初始化Authn", err))
		return
	}

	options, sessionData, err := instance.BeginLogin(expectedUser)

	if err != nil {
		c.JSON(200, ErrorResponse(err))
		return
	}

	val, err := json.Marshal(sessionData)
	if err != nil {
		c.JSON(200, ErrorResponse(err))
		return
	}

	util.SetSession(c, map[string]interface{}{
		"registration-session": val,
	})
	c.JSON(200, serializer.Response{Code: 0, Data: options})
}

// FinishLoginAuthn 完成註冊WebAuthn登入
func FinishLoginAuthn(c *gin.Context) {
	userName := c.Param("username")
	expectedUser, err := model.GetActiveUserByEmail(userName)
	if err != nil {
		c.JSON(200, serializer.Err(serializer.CodeCredentialInvalid, "使用者信箱或密碼錯誤", err))
		return
	}

	sessionDataJSON := util.GetSession(c, "registration-session").([]byte)

	var sessionData webauthn.SessionData
	err = json.Unmarshal(sessionDataJSON, &sessionData)

	instance, err := authn.NewAuthnInstance()
	if err != nil {
		c.JSON(200, serializer.Err(serializer.CodeInternalSetting, "無法初始化Authn", err))
		return
	}

	_, err = instance.FinishLogin(expectedUser, sessionData, c.Request)

	if err != nil {
		c.JSON(200, serializer.Err(serializer.CodeCredentialInvalid, "登入驗證失敗", err))
		return
	}

	util.SetSession(c, map[string]interface{}{
		"user_id": expectedUser.ID,
	})
	c.JSON(200, serializer.BuildUserResponse(expectedUser))
}

// StartRegAuthn 開始註冊WebAuthn訊息
func StartRegAuthn(c *gin.Context) {
	currUser := CurrentUser(c)

	instance, err := authn.NewAuthnInstance()
	if err != nil {
		c.JSON(200, serializer.Err(serializer.CodeInternalSetting, "無法初始化Authn", err))
		return
	}

	options, sessionData, err := instance.BeginRegistration(currUser)

	if err != nil {
		c.JSON(200, ErrorResponse(err))
		return
	}

	val, err := json.Marshal(sessionData)
	if err != nil {
		c.JSON(200, ErrorResponse(err))
		return
	}

	util.SetSession(c, map[string]interface{}{
		"registration-session": val,
	})
	c.JSON(200, serializer.Response{Code: 0, Data: options})
}

// FinishRegAuthn 完成註冊WebAuthn訊息
func FinishRegAuthn(c *gin.Context) {
	currUser := CurrentUser(c)
	sessionDataJSON := util.GetSession(c, "registration-session").([]byte)

	var sessionData webauthn.SessionData
	err := json.Unmarshal(sessionDataJSON, &sessionData)

	instance, err := authn.NewAuthnInstance()
	if err != nil {
		c.JSON(200, serializer.Err(serializer.CodeInternalSetting, "無法初始化Authn", err))
		return
	}

	credential, err := instance.FinishRegistration(currUser, sessionData, c.Request)

	if err != nil {
		c.JSON(200, ErrorResponse(err))
		return
	}

	err = currUser.RegisterAuthn(credential)
	if err != nil {
		c.JSON(200, ErrorResponse(err))
		return
	}

	c.JSON(200, serializer.Response{
		Code: 0,
		Data: map[string]interface{}{
			"id":          credential.ID,
			"fingerprint": fmt.Sprintf("% X", credential.Authenticator.AAGUID),
		},
	})
}

// UserLogin 使用者登入
func UserLogin(c *gin.Context) {
	var service user.UserLoginService
	if err := c.ShouldBindJSON(&service); err == nil {
		res := service.Login(c)
		c.JSON(200, res)
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}

// UserRegister 使用者註冊
func UserRegister(c *gin.Context) {
	var service user.UserRegisterService
	if err := c.ShouldBindJSON(&service); err == nil {
		res := service.Register(c)
		c.JSON(200, res)
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}

// User2FALogin 使用者二步驗證登入
func User2FALogin(c *gin.Context) {
	var service user.Enable2FA
	if err := c.ShouldBindJSON(&service); err == nil {
		res := service.Login(c)
		c.JSON(200, res)
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}

// UserSendReset 發送密碼重設郵件
func UserSendReset(c *gin.Context) {
	var service user.UserResetEmailService
	if err := c.ShouldBindJSON(&service); err == nil {
		res := service.Reset(c)
		c.JSON(200, res)
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}

// UserReset 重設密碼
func UserReset(c *gin.Context) {
	var service user.UserResetService
	if err := c.ShouldBindJSON(&service); err == nil {
		res := service.Reset(c)
		c.JSON(200, res)
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}

// UserActivate 使用者啟動
func UserActivate(c *gin.Context) {
	var service user.SettingService
	if err := c.ShouldBindUri(&service); err == nil {
		res := service.Activate(c)
		c.JSON(200, res)
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}

// UserSignOut 使用者退出登入
func UserSignOut(c *gin.Context) {
	util.DeleteSession(c, "user_id")
	c.JSON(200, serializer.Response{})
}

// UserMe 獲取目前登入的使用者
func UserMe(c *gin.Context) {
	currUser := CurrentUser(c)
	res := serializer.BuildUserResponse(*currUser)
	c.JSON(200, res)
}

// UserStorage 獲取使用者的儲存訊息
func UserStorage(c *gin.Context) {
	currUser := CurrentUser(c)
	res := serializer.BuildUserStorageResponse(*currUser)
	c.JSON(200, res)
}

// UserTasks 獲取任務佇列
func UserTasks(c *gin.Context) {
	var service user.SettingListService
	if err := c.ShouldBindQuery(&service); err == nil {
		res := service.ListTasks(c, CurrentUser(c))
		c.JSON(200, res)
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}

// UserSetting 獲取使用者設定
func UserSetting(c *gin.Context) {
	var service user.SettingService
	if err := c.ShouldBindUri(&service); err == nil {
		res := service.Settings(c, CurrentUser(c))
		c.JSON(200, res)
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}

// UseGravatar 設定大頭貼使用全球通用
func UseGravatar(c *gin.Context) {
	u := CurrentUser(c)
	if err := u.Update(map[string]interface{}{"avatar": "gravatar"}); err != nil {
		c.JSON(200, serializer.Err(serializer.CodeDBError, "無法更新大頭貼", err))
		return
	}
	c.JSON(200, serializer.Response{})
}

// UploadAvatar 從文件上傳大頭貼
func UploadAvatar(c *gin.Context) {
	// 取得大頭貼上傳大小限制
	maxSize := model.GetIntSetting("avatar_size", 2097152)
	if c.Request.ContentLength == -1 || c.Request.ContentLength > int64(maxSize) {
		request.BlackHole(c.Request.Body)
		c.JSON(200, serializer.Err(serializer.CodeUploadFailed, "大頭貼尺寸太大", nil))
		return
	}

	// 取得上傳的文件
	file, err := c.FormFile("avatar")
	if err != nil {
		c.JSON(200, serializer.Err(serializer.CodeIOFailed, "無法讀取大頭貼資料", err))
		return
	}

	// 初始化大頭貼
	r, err := file.Open()
	if err != nil {
		c.JSON(200, serializer.Err(serializer.CodeIOFailed, "無法讀取大頭貼資料", err))
		return
	}
	avatar, err := thumb.NewThumbFromFile(r, file.Filename)
	if err != nil {
		c.JSON(200, serializer.Err(serializer.CodeIOFailed, "無法解析圖像資料", err))
		return
	}

	// 建立大頭貼
	u := CurrentUser(c)
	err = avatar.CreateAvatar(u.ID)
	if err != nil {
		c.JSON(200, serializer.Err(serializer.CodeIOFailed, "無法建立大頭貼", err))
		return
	}

	// 儲存大頭貼標記
	if err := u.Update(map[string]interface{}{
		"avatar": "file",
	}); err != nil {
		c.JSON(200, serializer.Err(serializer.CodeDBError, "無法更新大頭貼", err))
		return
	}

	c.JSON(200, serializer.Response{})
}

// GetUserAvatar 獲取使用者大頭貼
func GetUserAvatar(c *gin.Context) {
	var service user.AvatarService
	if err := c.ShouldBindUri(&service); err == nil {
		res := service.Get(c)
		if res.Code == -301 {
			// 重定向到gravatar
			c.Redirect(301, res.Data.(string))
		}
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}

// UpdateOption 更改使用者設定
func UpdateOption(c *gin.Context) {
	var service user.SettingUpdateService
	if err := c.ShouldBindUri(&service); err == nil {
		var (
			subService user.OptionsChangeHandler
			subErr     error
		)

		switch service.Option {
		case "nick":
			subService = &user.ChangerNick{}
		case "homepage":
			subService = &user.HomePage{}
		case "password":
			subService = &user.PasswordChange{}
		case "2fa":
			subService = &user.Enable2FA{}
		case "authn":
			subService = &user.DeleteWebAuthn{}
		case "theme":
			subService = &user.ThemeChose{}
		default:
			subService = &user.ChangerNick{}
		}

		subErr = c.ShouldBindJSON(subService)
		if subErr != nil {
			c.JSON(200, ErrorResponse(subErr))
			return
		}

		res := subService.Update(c, CurrentUser(c))
		c.JSON(200, res)

	} else {
		c.JSON(200, ErrorResponse(err))
	}
}

// UserInit2FA 初始化二步驗證
func UserInit2FA(c *gin.Context) {
	var service user.SettingService
	if err := c.ShouldBindUri(&service); err == nil {
		res := service.Init2FA(c, CurrentUser(c))
		c.JSON(200, res)
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}
