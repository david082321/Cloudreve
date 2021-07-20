package setting

import (
	model "github.com/cloudreve/Cloudreve/v3/models"
	"github.com/cloudreve/Cloudreve/v3/pkg/serializer"
	"github.com/cloudreve/Cloudreve/v3/pkg/util"
	"github.com/gin-gonic/gin"
)

// WebDAVListService WebDAV 列表服務
type WebDAVListService struct {
}

// WebDAVAccountService WebDAV 帳號管理服務
type WebDAVAccountService struct {
	ID uint `uri:"id" binding:"required,min=1"`
}

// WebDAVAccountCreateService WebDAV 帳號建立服務
type WebDAVAccountCreateService struct {
	Path string `json:"path" binding:"required,min=1,max=65535"`
	Name string `json:"name" binding:"required,min=1,max=255"`
}

// WebDAVMountCreateService WebDAV 掛載建立服務
type WebDAVMountCreateService struct {
	Path   string `json:"path" binding:"required,min=1,max=65535"`
	Policy string `json:"policy" binding:"required,min=1"`
}

// Create 建立WebDAV帳戶
func (service *WebDAVAccountCreateService) Create(c *gin.Context, user *model.User) serializer.Response {
	account := model.Webdav{
		Name:     service.Name,
		Password: util.RandStringRunes(32),
		UserID:   user.ID,
		Root:     service.Path,
	}

	if _, err := account.Create(); err != nil {
		return serializer.Err(serializer.CodeDBError, "建立失敗", err)
	}

	return serializer.Response{
		Data: map[string]interface{}{
			"id":         account.ID,
			"password":   account.Password,
			"created_at": account.CreatedAt,
		},
	}
}

// Delete 刪除WebDAV帳戶
func (service *WebDAVAccountService) Delete(c *gin.Context, user *model.User) serializer.Response {
	model.DeleteWebDAVAccountByID(service.ID, user.ID)
	return serializer.Response{}
}

// Accounts 列出WebDAV帳號
func (service *WebDAVListService) Accounts(c *gin.Context, user *model.User) serializer.Response {
	accounts := model.ListWebDAVAccounts(user.ID)

	return serializer.Response{Data: map[string]interface{}{
		"accounts": accounts,
	}}
}
