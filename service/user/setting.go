package user

import (
	"crypto/md5"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	model "github.com/cloudreve/Cloudreve/v3/models"
	"github.com/cloudreve/Cloudreve/v3/pkg/serializer"
	"github.com/cloudreve/Cloudreve/v3/pkg/util"
	"github.com/gin-gonic/gin"
	"github.com/pquerna/otp/totp"
)

// SettingService 通用設定服務
type SettingService struct {
}

// SettingListService 通用設定列表服務
type SettingListService struct {
	Page int `form:"page" binding:"required,min=1"`
}

// AvatarService 大頭貼服務
type AvatarService struct {
	Size string `uri:"size" binding:"required,eq=l|eq=m|eq=s"`
}

// SettingUpdateService 設定更改服務
type SettingUpdateService struct {
	Option string `uri:"option" binding:"required,eq=nick|eq=theme|eq=homepage|eq=vip|eq=qq|eq=policy|eq=password|eq=2fa|eq=authn"`
}

// OptionsChangeHandler 屬性更改介面
type OptionsChangeHandler interface {
	Update(*gin.Context, *model.User) serializer.Response
}

// ChangerNick 暱稱更改服務
type ChangerNick struct {
	Nick string `json:"nick" binding:"required,min=1,max=255"`
}

// PolicyChange 更改儲存策略
type PolicyChange struct {
	ID string `json:"id" binding:"required"`
}

// HomePage 更改個人首頁開關
type HomePage struct {
	Enabled bool `json:"status"`
}

// PasswordChange 更改密碼
type PasswordChange struct {
	Old string `json:"old" binding:"required,min=4,max=64"`
	New string `json:"new" binding:"required,min=4,max=64"`
}

// Enable2FA 開啟二步驗證
type Enable2FA struct {
	Code string `json:"code" binding:"required"`
}

// DeleteWebAuthn 刪除WebAuthn憑證
type DeleteWebAuthn struct {
	ID string `json:"id" binding:"required"`
}

// ThemeChose 主題選擇
type ThemeChose struct {
	Theme string `json:"theme" binding:"required,hexcolor|rgb|rgba|hsl"`
}

// Update 更新主題設定
func (service *ThemeChose) Update(c *gin.Context, user *model.User) serializer.Response {
	user.OptionsSerialized.PreferredTheme = service.Theme
	if err := user.UpdateOptions(); err != nil {
		return serializer.DBErr("主題切換失敗", err)
	}

	return serializer.Response{}
}

// Update 刪除憑證
func (service *DeleteWebAuthn) Update(c *gin.Context, user *model.User) serializer.Response {
	user.RemoveAuthn(service.ID)
	return serializer.Response{}
}

// Update 更改二步驗證設定
func (service *Enable2FA) Update(c *gin.Context, user *model.User) serializer.Response {
	if user.TwoFactor == "" {
		// 開啟2FA
		secret, ok := util.GetSession(c, "2fa_init").(string)
		if !ok {
			return serializer.Err(serializer.CodeParamErr, "未初始化二步驗證", nil)
		}

		if !totp.Validate(service.Code, secret) {
			return serializer.ParamErr("驗證碼不正確", nil)
		}

		if err := user.Update(map[string]interface{}{"two_factor": secret}); err != nil {
			return serializer.DBErr("無法更新二步驗證設定", err)
		}

	} else {
		// 關閉2FA
		if !totp.Validate(service.Code, user.TwoFactor) {
			return serializer.ParamErr("驗證碼不正確", nil)
		}

		if err := user.Update(map[string]interface{}{"two_factor": ""}); err != nil {
			return serializer.DBErr("無法更新二步驗證設定", err)
		}
	}

	return serializer.Response{}
}

// Init2FA 初始化二步驗證
func (service *SettingService) Init2FA(c *gin.Context, user *model.User) serializer.Response {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "Cloudreve",
		AccountName: user.Email,
	})
	if err != nil {
		return serializer.Err(serializer.CodeInternalSetting, "無法生成驗金鑰", err)
	}

	util.SetSession(c, map[string]interface{}{"2fa_init": key.Secret()})
	return serializer.Response{Data: key.Secret()}
}

// Update 更改密碼
func (service *PasswordChange) Update(c *gin.Context, user *model.User) serializer.Response {
	// 驗證老密碼
	if ok, _ := user.CheckPassword(service.Old); !ok {
		return serializer.Err(serializer.CodeParamErr, "原密碼不正確", nil)
	}

	// 更改為新密碼
	user.SetPassword(service.New)
	if err := user.Update(map[string]interface{}{"password": user.Password}); err != nil {
		return serializer.DBErr("密碼更換失敗", err)
	}

	return serializer.Response{}
}

// Update 切換個人首頁開關
func (service *HomePage) Update(c *gin.Context, user *model.User) serializer.Response {
	user.OptionsSerialized.ProfileOff = !service.Enabled
	if err := user.UpdateOptions(); err != nil {
		return serializer.DBErr("儲存策略切換失敗", err)
	}

	return serializer.Response{}
}

// Update 更改暱稱
func (service *ChangerNick) Update(c *gin.Context, user *model.User) serializer.Response {
	if err := user.Update(map[string]interface{}{"nick": service.Nick}); err != nil {
		return serializer.DBErr("無法更新暱稱", err)
	}

	return serializer.Response{}
}

// Get 獲取使用者大頭貼
func (service *AvatarService) Get(c *gin.Context) serializer.Response {
	// 尋找目標使用者
	uid, _ := c.Get("object_id")
	user, err := model.GetActiveUserByID(uid.(uint))
	if err != nil {
		return serializer.Err(serializer.CodeNotFound, "使用者不存在", err)
	}

	// 未設定大頭貼時，返回404錯誤
	if user.Avatar == "" {
		c.Status(404)
		return serializer.Response{}
	}

	// 獲取大頭貼設定
	sizes := map[string]string{
		"s": model.GetSettingByName("avatar_size_s"),
		"m": model.GetSettingByName("avatar_size_m"),
		"l": model.GetSettingByName("avatar_size_l"),
	}

	// Gravatar 大頭貼重定向
	if user.Avatar == "gravatar" {
		server := model.GetSettingByName("gravatar_server")
		gravatarRoot, err := url.Parse(server)
		if err != nil {
			return serializer.Err(serializer.CodeInternalSetting, "無法解析 Gravatar 伺服器地址", err)
		}
		email_lowered := strings.ToLower(user.Email)
		has := md5.Sum([]byte(email_lowered))
		avatar, _ := url.Parse(fmt.Sprintf("/avatar/%x?d=mm&s=%s", has, sizes[service.Size]))

		return serializer.Response{
			Code: -301,
			Data: gravatarRoot.ResolveReference(avatar).String(),
		}
	}

	// 本機文件大頭貼
	if user.Avatar == "file" {
		avatarRoot := util.RelativePath(model.GetSettingByName("avatar_path"))
		sizeToInt := map[string]string{
			"s": "0",
			"m": "1",
			"l": "2",
		}

		avatar, err := os.Open(filepath.Join(avatarRoot, fmt.Sprintf("avatar_%d_%s.png", user.ID, sizeToInt[service.Size])))
		if err != nil {
			c.Status(404)
			return serializer.Response{}
		}
		defer avatar.Close()

		http.ServeContent(c.Writer, c.Request, "avatar.png", user.UpdatedAt, avatar)
		return serializer.Response{}
	}

	c.Status(404)
	return serializer.Response{}
}

// ListTasks 列出任務
func (service *SettingListService) ListTasks(c *gin.Context, user *model.User) serializer.Response {
	tasks, total := model.ListTasks(user.ID, service.Page, 10, "updated_at desc")
	return serializer.BuildTaskList(tasks, total)
}

// Settings 獲取使用者設定
func (service *SettingService) Settings(c *gin.Context, user *model.User) serializer.Response {
	return serializer.Response{
		Data: map[string]interface{}{
			"uid":          user.ID,
			"homepage":     !user.OptionsSerialized.ProfileOff,
			"two_factor":   user.TwoFactor != "",
			"prefer_theme": user.OptionsSerialized.PreferredTheme,
			"themes":       model.GetSettingByName("themes"),
			"authn":        serializer.BuildWebAuthnList(user.WebAuthnCredentials()),
		},
	}
}
