package admin

import (
	"fmt"

	model "github.com/cloudreve/Cloudreve/v3/models"
	"github.com/cloudreve/Cloudreve/v3/pkg/serializer"
)

// AddGroupService 使用者群組添加服務
type AddGroupService struct {
	Group model.Group `json:"group" binding:"required"`
}

// GroupService 使用者群組ID服務
type GroupService struct {
	ID uint `uri:"id" json:"id" binding:"required"`
}

// Get 獲取使用者群組詳情
func (service *GroupService) Get() serializer.Response {
	group, err := model.GetGroupByID(service.ID)
	if err != nil {
		return serializer.Err(serializer.CodeNotFound, "儲存策略不存在", err)
	}

	return serializer.Response{Data: group}
}

// Delete 刪除使用者群組
func (service *GroupService) Delete() serializer.Response {
	// 尋找使用者群組
	group, err := model.GetGroupByID(service.ID)
	if err != nil {
		return serializer.Err(serializer.CodeNotFound, "使用者群組不存在", err)
	}

	// 是否為系統使用者群組
	if group.ID <= 3 {
		return serializer.Err(serializer.CodeNoPermissionErr, "系統使用者群組無法刪除", err)
	}

	// 檢查是否有使用者使用
	total := 0
	row := model.DB.Model(&model.User{}).Where("group_id = ?", service.ID).
		Select("count(id)").Row()
	row.Scan(&total)
	if total > 0 {
		return serializer.ParamErr(fmt.Sprintf("有 %d 位使用者仍屬於此使用者群組，請先刪除這些使用者或者更改使用者群組", total), nil)
	}

	model.DB.Delete(&group)

	return serializer.Response{}
}

// Add 添加使用者群組
func (service *AddGroupService) Add() serializer.Response {
	if service.Group.ID > 0 {
		if err := model.DB.Save(&service.Group).Error; err != nil {
			return serializer.ParamErr("使用者群組儲存失敗", err)
		}
	} else {
		if err := model.DB.Create(&service.Group).Error; err != nil {
			return serializer.ParamErr("使用者群組添加失敗", err)
		}
	}

	return serializer.Response{Data: service.Group.ID}
}

// Groups 列出使用者群組
func (service *AdminListService) Groups() serializer.Response {
	var res []model.Group
	total := 0

	tx := model.DB.Model(&model.Group{})
	if service.OrderBy != "" {
		tx = tx.Order(service.OrderBy)
	}

	for k, v := range service.Conditions {
		tx = tx.Where(k+" = ?", v)
	}

	// 計算總數用於分頁
	tx.Count(&total)

	// 查詢記錄
	tx.Limit(service.PageSize).Offset((service.Page - 1) * service.PageSize).Find(&res)

	// 統計每個使用者群組的使用者總數
	statics := make(map[uint]int, len(res))
	for i := 0; i < len(res); i++ {
		total := 0
		row := model.DB.Model(&model.User{}).Where("group_id = ?", res[i].ID).
			Select("count(id)").Row()
		row.Scan(&total)
		statics[res[i].ID] = total
	}

	// 匯總使用者群組儲存策略
	policies := make(map[uint]model.Policy)
	for i := 0; i < len(res); i++ {
		for _, p := range res[i].PolicyList {
			if _, ok := policies[p]; !ok {
				policies[p], _ = model.GetPolicyByID(p)
			}
		}
	}

	return serializer.Response{Data: map[string]interface{}{
		"total":    total,
		"items":    res,
		"statics":  statics,
		"policies": policies,
	}}
}
