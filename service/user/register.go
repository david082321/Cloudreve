package user

import (
	model "github.com/cloudreve/Cloudreve/v3/models"
	"github.com/cloudreve/Cloudreve/v3/pkg/auth"
	"github.com/cloudreve/Cloudreve/v3/pkg/email"
	"github.com/cloudreve/Cloudreve/v3/pkg/hashid"
	"github.com/cloudreve/Cloudreve/v3/pkg/serializer"
	"github.com/gin-gonic/gin"
	"net/url"
	"strings"
)

// UserRegisterService 管理使用者註冊的服務
type UserRegisterService struct {
	//TODO 細緻調整驗證規則
	UserName string `form:"userName" json:"userName" binding:"required,email"`
	Password string `form:"Password" json:"Password" binding:"required,min=4,max=64"`
}

// Register 新使用者註冊
func (service *UserRegisterService) Register(c *gin.Context) serializer.Response {
	// 相關設定
	options := model.GetSettingByNames("email_active")

	// 相關設定
	isEmailRequired := model.IsTrueVal(options["email_active"])
	defaultGroup := model.GetIntSetting("default_group", 2)

	// 建立新的使用者物件
	user := model.NewUser()
	user.Email = service.UserName
	user.Nick = strings.Split(service.UserName, "@")[0]
	user.SetPassword(service.Password)
	user.Status = model.Active
	if isEmailRequired {
		user.Status = model.NotActivicated
	}
	user.GroupID = uint(defaultGroup)
	userNotActivated := false
	// 建立使用者
	if err := model.DB.Create(&user).Error; err != nil {
		//檢查已存在使用者是否尚未啟動
		expectedUser, err := model.GetUserByEmail(service.UserName)
		if expectedUser.Status == model.NotActivicated {
			userNotActivated = true
			user = expectedUser
		} else {
			return serializer.DBErr("此信箱已被使用", err)
		}
	}

	// 發送啟動郵件
	if isEmailRequired {

		// 簽名啟動請求API
		base := model.GetSiteURL()
		userID := hashid.HashID(user.ID, hashid.UserID)
		controller, _ := url.Parse("/api/v3/user/activate/" + userID)
		activateURL, err := auth.SignURI(auth.General, base.ResolveReference(controller).String(), 86400)
		if err != nil {
			return serializer.Err(serializer.CodeEncryptError, "無法簽名啟動URL", err)
		}

		// 取得簽名
		credential := activateURL.Query().Get("sign")

		// 生成對使用者訪問的啟動地址
		controller, _ = url.Parse("/activate")
		finalURL := base.ResolveReference(controller)
		queries := finalURL.Query()
		queries.Add("id", userID)
		queries.Add("sign", credential)
		finalURL.RawQuery = queries.Encode()

		// 返送啟動郵件
		title, body := email.NewActivationEmail(user.Email,
			finalURL.String(),
		)
		if err := email.Send(user.Email, title, body); err != nil {
			return serializer.Err(serializer.CodeInternalSetting, "無法發送啟動郵件", err)
		}
		if userNotActivated == true {
			//原本在上面要拋出的DBErr，放來這邊拋出
			return serializer.DBErr("使用者未啟動，已重新髮送啟動郵件", nil)
		} else {
			return serializer.Response{Code: 203}
		}
	}

	return serializer.Response{}
}

// Activate 啟動使用者
func (service *SettingService) Activate(c *gin.Context) serializer.Response {
	// 尋找待啟動使用者
	uid, _ := c.Get("object_id")
	user, err := model.GetUserByID(uid.(uint))
	if err != nil {
		return serializer.Err(serializer.CodeNotFound, "使用者不存在", err)
	}

	// 檢查狀態
	if user.Status != model.NotActivicated {
		return serializer.Err(serializer.CodeNoPermissionErr, "該使用者無法被啟動", nil)
	}

	// 啟動使用者
	user.SetStatus(model.Active)

	return serializer.Response{Data: user.Email}
}
