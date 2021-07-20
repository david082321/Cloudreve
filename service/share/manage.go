package share

import (
	"net/url"
	"time"

	model "github.com/cloudreve/Cloudreve/v3/models"
	"github.com/cloudreve/Cloudreve/v3/pkg/hashid"
	"github.com/cloudreve/Cloudreve/v3/pkg/serializer"
	"github.com/gin-gonic/gin"
)

// ShareCreateService 建立新分享服務
type ShareCreateService struct {
	SourceID        string `json:"id" binding:"required"`
	IsDir           bool   `json:"is_dir"`
	Password        string `json:"password" binding:"max=255"`
	RemainDownloads int    `json:"downloads"`
	Expire          int    `json:"expire"`
	Preview         bool   `json:"preview"`
}

// ShareUpdateService 分享更新服務
type ShareUpdateService struct {
	Prop  string `json:"prop" binding:"required,eq=password|eq=preview_enabled"`
	Value string `json:"value" binding:"max=255"`
}

// Delete 刪除分享
func (service *Service) Delete(c *gin.Context, user *model.User) serializer.Response {
	share := model.GetShareByHashID(c.Param("id"))
	if share == nil || share.Creator().ID != user.ID {
		return serializer.Err(serializer.CodeNotFound, "分享不存在", nil)
	}

	if err := share.Delete(); err != nil {
		return serializer.Err(serializer.CodeDBError, "分享刪除失敗", err)
	}

	return serializer.Response{}
}

// Update 更新分享屬性
func (service *ShareUpdateService) Update(c *gin.Context) serializer.Response {
	shareCtx, _ := c.Get("share")
	share := shareCtx.(*model.Share)

	switch service.Prop {
	case "password":
		err := share.Update(map[string]interface{}{"password": service.Value})
		if err != nil {
			return serializer.Err(serializer.CodeDBError, "無法更新分享密碼", err)
		}
	case "preview_enabled":
		value := service.Value == "true"
		err := share.Update(map[string]interface{}{"preview_enabled": value})
		if err != nil {
			return serializer.Err(serializer.CodeDBError, "無法更新分享屬性", err)
		}
		return serializer.Response{
			Data: value,
		}
	}
	return serializer.Response{
		Data: service.Value,
	}
}

// Create 建立新分享
func (service *ShareCreateService) Create(c *gin.Context) serializer.Response {
	userCtx, _ := c.Get("user")
	user := userCtx.(*model.User)

	// 是否擁有權限
	if !user.Group.ShareEnabled {
		return serializer.Err(serializer.CodeNoPermissionErr, "您無權建立分享連結", nil)
	}

	// 源物件真實ID
	var (
		sourceID   uint
		sourceName string
		err        error
	)
	if service.IsDir {
		sourceID, err = hashid.DecodeHashID(service.SourceID, hashid.FolderID)
	} else {
		sourceID, err = hashid.DecodeHashID(service.SourceID, hashid.FileID)
	}
	if err != nil {
		return serializer.Err(serializer.CodeNotFound, "原始資源不存在", nil)
	}

	// 物件是否存在
	exist := true
	if service.IsDir {
		folder, err := model.GetFoldersByIDs([]uint{sourceID}, user.ID)
		if err != nil || len(folder) == 0 {
			exist = false
		} else {
			sourceName = folder[0].Name
		}
	} else {
		file, err := model.GetFilesByIDs([]uint{sourceID}, user.ID)
		if err != nil || len(file) == 0 {
			exist = false
		} else {
			sourceName = file[0].Name
		}
	}
	if !exist {
		return serializer.Err(serializer.CodeNotFound, "原始資源不存在", nil)
	}

	newShare := model.Share{
		Password:        service.Password,
		IsDir:           service.IsDir,
		UserID:          user.ID,
		SourceID:        sourceID,
		RemainDownloads: -1,
		PreviewEnabled:  service.Preview,
		SourceName:      sourceName,
	}

	// 如果開啟了自動過期
	if service.RemainDownloads > 0 {
		expires := time.Now().Add(time.Duration(service.Expire) * time.Second)
		newShare.RemainDownloads = service.RemainDownloads
		newShare.Expires = &expires
	}

	// 建立分享
	id, err := newShare.Create()
	if err != nil {
		return serializer.Err(serializer.CodeDBError, "分享連結建立失敗", err)
	}

	// 獲取分享的唯一id
	uid := hashid.HashID(id, hashid.ShareID)
	// 最終得到分享連結
	siteURL := model.GetSiteURL()
	sharePath, _ := url.Parse("/s/" + uid)
	shareURL := siteURL.ResolveReference(sharePath)

	return serializer.Response{
		Code: 0,
		Data: shareURL.String(),
	}

}
