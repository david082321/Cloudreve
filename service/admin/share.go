package admin

import (
	"strings"

	model "github.com/cloudreve/Cloudreve/v3/models"
	"github.com/cloudreve/Cloudreve/v3/pkg/hashid"
	"github.com/cloudreve/Cloudreve/v3/pkg/serializer"
	"github.com/gin-gonic/gin"
)

// ShareBatchService 分享批次操作服務
type ShareBatchService struct {
	ID []uint `json:"id" binding:"min=1"`
}

// Delete 刪除文件
func (service *ShareBatchService) Delete(c *gin.Context) serializer.Response {
	if err := model.DB.Where("id in (?)", service.ID).Delete(&model.Share{}).Error; err != nil {
		return serializer.DBErr("無法刪除分享", err)
	}
	return serializer.Response{}
}

// Shares 列出分享
func (service *AdminListService) Shares() serializer.Response {
	var res []model.Share
	total := 0

	tx := model.DB.Model(&model.Share{})
	if service.OrderBy != "" {
		tx = tx.Order(service.OrderBy)
	}

	for k, v := range service.Conditions {
		tx = tx.Where(k+" = ?", v)
	}

	if len(service.Searches) > 0 {
		search := ""
		for k, v := range service.Searches {
			search += k + " like '%" + v + "%' OR "
		}
		search = strings.TrimSuffix(search, " OR ")
		tx = tx.Where(search)
	}

	// 計算總數用於分頁
	tx.Count(&total)

	// 查詢記錄
	tx.Limit(service.PageSize).Offset((service.Page - 1) * service.PageSize).Find(&res)

	// 查詢對應使用者，同時計算HashID
	users := make(map[uint]model.User)
	hashIDs := make(map[uint]string, len(res))
	for _, file := range res {
		users[file.UserID] = model.User{}
		hashIDs[file.ID] = hashid.HashID(file.ID, hashid.ShareID)
	}

	userIDs := make([]uint, 0, len(users))
	for k := range users {
		userIDs = append(userIDs, k)
	}

	var userList []model.User
	model.DB.Where("id in (?)", userIDs).Find(&userList)

	for _, v := range userList {
		users[v.ID] = v
	}

	return serializer.Response{Data: map[string]interface{}{
		"total": total,
		"items": res,
		"users": users,
		"ids":   hashIDs,
	}}
}
