package admin

import (
	"context"
	"strings"

	model "github.com/cloudreve/Cloudreve/v3/models"
	"github.com/cloudreve/Cloudreve/v3/pkg/filesystem"
	"github.com/cloudreve/Cloudreve/v3/pkg/serializer"
)

// AddUserService 使用者添加服務
type AddUserService struct {
	User     model.User `json:"User" binding:"required"`
	Password string     `json:"password"`
}

// UserService 使用者ID服務
type UserService struct {
	ID uint `uri:"id" json:"id" binding:"required"`
}

// UserBatchService 使用者批次操作服務
type UserBatchService struct {
	ID []uint `json:"id" binding:"min=1"`
}

// Ban 封禁/解封使用者
func (service *UserService) Ban() serializer.Response {
	user, err := model.GetUserByID(service.ID)
	if err != nil {
		return serializer.Err(serializer.CodeNotFound, "使用者不存在", err)
	}

	if user.ID == 1 {
		return serializer.Err(serializer.CodeNoPermissionErr, "無法封禁初始使用者", err)
	}

	if user.Status == model.Active {
		user.SetStatus(model.Baned)
	} else {
		user.SetStatus(model.Active)
	}

	return serializer.Response{Data: user.Status}
}

// Delete 刪除使用者
func (service *UserBatchService) Delete() serializer.Response {
	for _, uid := range service.ID {
		user, err := model.GetUserByID(uid)
		if err != nil {
			return serializer.Err(serializer.CodeNotFound, "使用者不存在", err)
		}

		// 不能刪除初始使用者
		if uid == 1 {
			return serializer.Err(serializer.CodeNoPermissionErr, "無法刪除初始使用者", err)
		}

		// 刪除與此使用者相關的所有資源

		fs, err := filesystem.NewFileSystem(&user)
		// 刪除所有文件
		root, err := fs.User.Root()
		if err != nil {
			return serializer.Err(serializer.CodeNotFound, "無法找到使用者根目錄", err)
		}
		fs.Delete(context.Background(), []uint{root.ID}, []uint{}, false)

		// 刪除相關任務
		model.DB.Where("user_id = ?", uid).Delete(&model.Download{})
		model.DB.Where("user_id = ?", uid).Delete(&model.Task{})

		// 刪除標籤
		model.DB.Where("user_id = ?", uid).Delete(&model.Tag{})

		// 刪除WebDAV帳號
		model.DB.Where("user_id = ?", uid).Delete(&model.Webdav{})

		// 刪除此使用者
		model.DB.Unscoped().Delete(user)

	}
	return serializer.Response{}
}

// Get 獲取使用者詳情
func (service *UserService) Get() serializer.Response {
	group, err := model.GetUserByID(service.ID)
	if err != nil {
		return serializer.Err(serializer.CodeNotFound, "使用者不存在", err)
	}

	return serializer.Response{Data: group}
}

// Add 添加使用者
func (service *AddUserService) Add() serializer.Response {
	if service.User.ID > 0 {

		user, _ := model.GetUserByID(service.User.ID)
		if service.Password != "" {
			user.SetPassword(service.Password)
		}

		// 只更新必要欄位
		user.Nick = service.User.Nick
		user.Email = service.User.Email
		user.GroupID = service.User.GroupID
		user.Status = service.User.Status

		// 檢查愚蠢操作
		if user.ID == 1 && user.GroupID != 1 {
			return serializer.ParamErr("無法更改初始使用者的使用者群組", nil)
		}

		if err := model.DB.Save(&user).Error; err != nil {
			return serializer.ParamErr("使用者儲存失敗", err)
		}
	} else {
		service.User.SetPassword(service.Password)
		if err := model.DB.Create(&service.User).Error; err != nil {
			return serializer.ParamErr("使用者群組添加失敗", err)
		}
	}

	return serializer.Response{Data: service.User.ID}
}

// Users 列出使用者
func (service *AdminListService) Users() serializer.Response {
	var res []model.User
	total := 0

	tx := model.DB.Model(&model.User{})
	if service.OrderBy != "" {
		tx = tx.Order(service.OrderBy)
	}

	for k, v := range service.Conditions {
		tx = tx.Where(k+" = ?", v)
	}

	if len(service.Searches) > 0 {
		search := ""
		for k, v := range service.Searches {
			search += (k + " like '%" + v + "%' OR ")
		}
		search = strings.TrimSuffix(search, " OR ")
		tx = tx.Where(search)
	}

	// 計算總數用於分頁
	tx.Count(&total)

	// 查詢記錄
	tx.Set("gorm:auto_preload", true).Limit(service.PageSize).Offset((service.Page - 1) * service.PageSize).Find(&res)

	// 補齊缺失使用者群組

	return serializer.Response{Data: map[string]interface{}{
		"total": total,
		"items": res,
	}}
}
