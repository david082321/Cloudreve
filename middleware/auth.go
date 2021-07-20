package middleware

import (
	"bytes"
	"context"
	"crypto/md5"
	"fmt"
	"io/ioutil"
	"net/http"

	model "github.com/cloudreve/Cloudreve/v3/models"
	"github.com/cloudreve/Cloudreve/v3/pkg/auth"
	"github.com/cloudreve/Cloudreve/v3/pkg/cache"
	"github.com/cloudreve/Cloudreve/v3/pkg/filesystem/driver/onedrive"
	"github.com/cloudreve/Cloudreve/v3/pkg/filesystem/driver/oss"
	"github.com/cloudreve/Cloudreve/v3/pkg/filesystem/driver/upyun"
	"github.com/cloudreve/Cloudreve/v3/pkg/serializer"
	"github.com/cloudreve/Cloudreve/v3/pkg/util"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/qiniu/api.v7/v7/auth/qbox"
)

// SignRequired 驗證請求簽名
func SignRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		var err error
		switch c.Request.Method {
		case "PUT", "POST":
			err = auth.CheckRequest(auth.General, c.Request)
			// TODO 生產環境去掉下一行
			//err = nil
		default:
			err = auth.CheckURI(auth.General, c.Request.URL)
		}

		if err != nil {
			c.JSON(200, serializer.Err(serializer.CodeCredentialInvalid, err.Error(), err))
			c.Abort()
			return
		}
		c.Next()
	}
}

// CurrentUser 獲取登入使用者
func CurrentUser() gin.HandlerFunc {
	return func(c *gin.Context) {
		session := sessions.Default(c)
		uid := session.Get("user_id")
		if uid != nil {
			user, err := model.GetActiveUserByID(uid)
			if err == nil {
				c.Set("user", &user)
			}
		}
		c.Next()
	}
}

// AuthRequired 需要登入
func AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		if user, _ := c.Get("user"); user != nil {
			if _, ok := user.(*model.User); ok {
				c.Next()
				return
			}
		}

		c.JSON(200, serializer.CheckLogin())
		c.Abort()
	}
}

// WebDAVAuth 驗證WebDAV登入及權限
func WebDAVAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// OPTIONS 請求不需要鑒權，否則Windows10下無法儲存文件
		if c.Request.Method == "OPTIONS" {
			c.Next()
			return
		}

		username, password, ok := c.Request.BasicAuth()
		if !ok {
			c.Writer.Header()["WWW-Authenticate"] = []string{`Basic realm="cloudreve"`}
			c.Status(http.StatusUnauthorized)
			c.Abort()
			return
		}

		expectedUser, err := model.GetActiveUserByEmail(username)
		if err != nil {
			c.Status(http.StatusUnauthorized)
			c.Abort()
			return
		}

		// 密碼正確？
		webdav, err := model.GetWebdavByPassword(password, expectedUser.ID)
		if err != nil {
			c.Status(http.StatusUnauthorized)
			c.Abort()
			return
		}

		// 使用者群組已啟用WebDAV？
		if !expectedUser.Group.WebDAVEnabled {
			c.Status(http.StatusForbidden)
			c.Abort()
			return
		}

		c.Set("user", &expectedUser)
		c.Set("webdav", webdav)
		c.Next()
	}
}

// uploadCallbackCheck 對上傳回調請求的 callback key 進行驗證，如果成功則返回上傳使用者
func uploadCallbackCheck(c *gin.Context) (serializer.Response, *model.User) {
	// 驗證 Callback Key
	callbackKey := c.Param("key")
	if callbackKey == "" {
		return serializer.ParamErr("Callback Key 不能為空", nil), nil
	}
	callbackSessionRaw, exist := cache.Get("callback_" + callbackKey)
	if !exist {
		return serializer.ParamErr("回調工作階段不存在或已過期", nil), nil
	}
	callbackSession := callbackSessionRaw.(serializer.UploadSession)
	c.Set("callbackSession", &callbackSession)

	// 清理回調工作階段
	_ = cache.Deletes([]string{callbackKey}, "callback_")

	// 尋找使用者
	user, err := model.GetActiveUserByID(callbackSession.UID)
	if err != nil {
		return serializer.Err(serializer.CodeCheckLogin, "找不到使用者", err), nil
	}
	c.Set("user", &user)

	return serializer.Response{}, &user
}

// RemoteCallbackAuth 遠端回調簽名驗證
func RemoteCallbackAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 驗證key並尋找使用者
		resp, user := uploadCallbackCheck(c)
		if resp.Code != 0 {
			c.JSON(200, resp)
			c.Abort()
			return
		}

		// 驗證簽名
		authInstance := auth.HMACAuth{SecretKey: []byte(user.Policy.SecretKey)}
		if err := auth.CheckRequest(authInstance, c.Request); err != nil {
			c.JSON(200, serializer.Err(serializer.CodeCheckLogin, err.Error(), err))
			c.Abort()
			return
		}

		c.Next()

	}
}

// QiniuCallbackAuth 七牛回調簽名驗證
func QiniuCallbackAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 驗證key並尋找使用者
		resp, user := uploadCallbackCheck(c)
		if resp.Code != 0 {
			c.JSON(401, serializer.GeneralUploadCallbackFailed{Error: resp.Msg})
			c.Abort()
			return
		}

		// 驗證回調是否來自qiniu
		mac := qbox.NewMac(user.Policy.AccessKey, user.Policy.SecretKey)
		ok, err := mac.VerifyCallback(c.Request)
		if err != nil {
			util.Log().Debug("無法驗證回調請求，%s", err)
			c.JSON(401, serializer.GeneralUploadCallbackFailed{Error: "無法驗證回調請求"})
			c.Abort()
			return
		}
		if !ok {
			c.JSON(401, serializer.GeneralUploadCallbackFailed{Error: "回調簽名無效"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// OSSCallbackAuth 阿里雲OSS回調簽名驗證
func OSSCallbackAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 驗證key並尋找使用者
		resp, _ := uploadCallbackCheck(c)
		if resp.Code != 0 {
			c.JSON(401, serializer.GeneralUploadCallbackFailed{Error: resp.Msg})
			c.Abort()
			return
		}

		err := oss.VerifyCallbackSignature(c.Request)
		if err != nil {
			util.Log().Debug("回調簽名驗證失敗，%s", err)
			c.JSON(401, serializer.GeneralUploadCallbackFailed{Error: "回調簽名驗證失敗"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// UpyunCallbackAuth 又拍雲回調簽名驗證
func UpyunCallbackAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 驗證key並尋找使用者
		resp, user := uploadCallbackCheck(c)
		if resp.Code != 0 {
			c.JSON(401, serializer.GeneralUploadCallbackFailed{Error: resp.Msg})
			c.Abort()
			return
		}

		// 獲取請求正文
		body, err := ioutil.ReadAll(c.Request.Body)
		c.Request.Body.Close()
		if err != nil {
			c.JSON(401, serializer.GeneralUploadCallbackFailed{Error: err.Error()})
			c.Abort()
			return
		}

		c.Request.Body = ioutil.NopCloser(bytes.NewReader(body))

		// 準備驗證Upyun回調簽名
		handler := upyun.Driver{Policy: &user.Policy}
		contentMD5 := c.Request.Header.Get("Content-Md5")
		date := c.Request.Header.Get("Date")
		actualSignature := c.Request.Header.Get("Authorization")

		// 計算正文MD5
		actualContentMD5 := fmt.Sprintf("%x", md5.Sum(body))
		if actualContentMD5 != contentMD5 {
			c.JSON(401, serializer.GeneralUploadCallbackFailed{Error: "MD5不一致"})
			c.Abort()
			return
		}

		// 計算理論簽名
		signature := handler.Sign(context.Background(), []string{
			"POST",
			c.Request.URL.Path,
			date,
			contentMD5,
		})

		// 對比簽名
		if signature != actualSignature {
			c.JSON(401, serializer.GeneralUploadCallbackFailed{Error: "鑒權失敗"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// OneDriveCallbackAuth OneDrive回調簽名驗證
// TODO 解耦
func OneDriveCallbackAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 驗證key並尋找使用者
		resp, _ := uploadCallbackCheck(c)
		if resp.Code != 0 {
			c.JSON(401, serializer.GeneralUploadCallbackFailed{Error: resp.Msg})
			c.Abort()
			return
		}

		// 發送回調結束訊號
		onedrive.FinishCallback(c.Param("key"))

		c.Next()
	}
}

// COSCallbackAuth 騰訊雲COS回調簽名驗證
// TODO 解耦 測試
func COSCallbackAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 驗證key並尋找使用者
		resp, _ := uploadCallbackCheck(c)
		if resp.Code != 0 {
			c.JSON(401, serializer.GeneralUploadCallbackFailed{Error: resp.Msg})
			c.Abort()
			return
		}

		c.Next()
	}
}

// S3CallbackAuth Amazon S3回調簽名驗證
func S3CallbackAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 驗證key並尋找使用者
		resp, _ := uploadCallbackCheck(c)
		if resp.Code != 0 {
			c.JSON(401, serializer.GeneralUploadCallbackFailed{Error: resp.Msg})
			c.Abort()
			return
		}

		c.Next()
	}
}

// IsAdmin 必須為管理員使用者群組
func IsAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		user, _ := c.Get("user")
		if user.(*model.User).Group.ID != 1 && user.(*model.User).ID != 1 {
			c.JSON(200, serializer.Err(serializer.CodeAdminRequired, "您不是管理組成員", nil))
			c.Abort()
			return
		}

		c.Next()
	}
}
