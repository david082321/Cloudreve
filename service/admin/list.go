package admin

import (
	model "github.com/cloudreve/Cloudreve/v3/models"
	"github.com/cloudreve/Cloudreve/v3/pkg/serializer"
)

// AdminListService 儀表盤列條目服務
type AdminListService struct {
	Page       int               `json:"page" binding:"min=1,required"`
	PageSize   int               `json:"page_size" binding:"min=1,required"`
	OrderBy    string            `json:"order_by"`
	Conditions map[string]string `form:"conditions"`
	Searches   map[string]string `form:"searches"`
}

// GroupList 獲取使用者群組列表
func (service *NoParamService) GroupList() serializer.Response {
	var res []model.Group
	model.DB.Model(&model.Group{}).Find(&res)
	return serializer.Response{Data: res}
}
