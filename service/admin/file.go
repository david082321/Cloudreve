package admin

import (
	"context"
	"strings"

	model "github.com/cloudreve/Cloudreve/v3/models"
	"github.com/cloudreve/Cloudreve/v3/pkg/filesystem"
	"github.com/cloudreve/Cloudreve/v3/pkg/filesystem/fsctx"
	"github.com/cloudreve/Cloudreve/v3/pkg/serializer"
	"github.com/cloudreve/Cloudreve/v3/service/explorer"
	"github.com/gin-gonic/gin"
)

// FileService 文件ID服務
type FileService struct {
	ID uint `uri:"id" json:"id" binding:"required"`
}

// FileBatchService 文件批次操作服務
type FileBatchService struct {
	ID    []uint `json:"id" binding:"min=1"`
	Force bool   `json:"force"`
}

// ListFolderService 列目錄結構
type ListFolderService struct {
	Path string `uri:"path" binding:"required,max=65535"`
	ID   uint   `uri:"id" binding:"required"`
	Type string `uri:"type" binding:"eq=policy|eq=user"`
}

// List 列出指定路徑下的目錄
func (service *ListFolderService) List(c *gin.Context) serializer.Response {
	if service.Type == "policy" {
		// 列取儲存策略中的目錄
		policy, err := model.GetPolicyByID(service.ID)
		if err != nil {
			return serializer.Err(serializer.CodeNotFound, "儲存策略不存在", err)
		}

		// 建立文件系統
		fs, err := filesystem.NewAnonymousFileSystem()
		if err != nil {
			return serializer.Err(serializer.CodeInternalSetting, "無法建立文件系統", err)
		}
		defer fs.Recycle()

		// 列取儲存策略中的文件
		fs.Policy = &policy
		res, err := fs.ListPhysical(c.Request.Context(), service.Path)
		if err != nil {
			return serializer.Err(serializer.CodeIOFailed, "無法列取目錄", err)
		}

		return serializer.Response{
			Data: map[string]interface{}{
				"objects": res,
			},
		}

	}

	// 列取使用者空間目錄
	// 尋找使用者
	user, err := model.GetUserByID(service.ID)
	if err != nil {
		return serializer.Err(serializer.CodeNotFound, "使用者不存在", err)
	}

	// 建立文件系統
	fs, err := filesystem.NewFileSystem(&user)
	if err != nil {
		return serializer.Err(serializer.CodeInternalSetting, "無法建立文件系統", err)
	}
	defer fs.Recycle()

	// 列取目錄
	res, err := fs.List(c.Request.Context(), service.Path, nil)
	if err != nil {
		return serializer.Err(serializer.CodeIOFailed, "無法列取目錄", err)
	}

	return serializer.Response{
		Data: map[string]interface{}{
			"objects": res,
		},
	}
}

// Delete 刪除文件
func (service *FileBatchService) Delete(c *gin.Context) serializer.Response {
	files, err := model.GetFilesByIDs(service.ID, 0)
	if err != nil {
		return serializer.DBErr("無法列出待刪除文件", err)
	}

	// 根據使用者分組
	userFile := make(map[uint][]model.File)
	for i := 0; i < len(files); i++ {
		if _, ok := userFile[files[i].UserID]; !ok {
			userFile[files[i].UserID] = []model.File{}
		}
		userFile[files[i].UserID] = append(userFile[files[i].UserID], files[i])
	}

	// 非同步執行刪除
	go func(files map[uint][]model.File) {
		for uid, file := range files {
			user, err := model.GetUserByID(uid)
			if err != nil {
				continue
			}

			fs, err := filesystem.NewFileSystem(&user)
			if err != nil {
				fs.Recycle()
				continue
			}

			// 匯總文件ID
			ids := make([]uint, 0, len(file))
			for i := 0; i < len(file); i++ {
				ids = append(ids, file[i].ID)
			}

			// 執行刪除
			fs.Delete(context.Background(), []uint{}, ids, service.Force)
			fs.Recycle()
		}
	}(userFile)

	// 分組執行刪除
	return serializer.Response{}

}

// Get 預覽文件
func (service *FileService) Get(c *gin.Context) serializer.Response {
	file, err := model.GetFilesByIDs([]uint{service.ID}, 0)
	if err != nil {
		return serializer.Err(serializer.CodeNotFound, "文件不存在", err)
	}

	ctx := context.WithValue(context.Background(), fsctx.FileModelCtx, &file[0])
	var subService explorer.FileIDService
	res := subService.PreviewContent(ctx, c, false)

	return res
}

// Files 列出文件
func (service *AdminListService) Files() serializer.Response {
	var res []model.File
	total := 0

	tx := model.DB.Model(&model.File{})
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

	// 查詢對應使用者
	users := make(map[uint]model.User)
	for _, file := range res {
		users[file.UserID] = model.User{}
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
	}}
}
