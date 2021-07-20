package explorer

import (
	"context"
	"fmt"
	"math"
	"net/url"
	"path"
	"strings"
	"time"

	model "github.com/cloudreve/Cloudreve/v3/models"
	"github.com/cloudreve/Cloudreve/v3/pkg/auth"
	"github.com/cloudreve/Cloudreve/v3/pkg/cache"
	"github.com/cloudreve/Cloudreve/v3/pkg/filesystem"
	"github.com/cloudreve/Cloudreve/v3/pkg/filesystem/fsctx"
	"github.com/cloudreve/Cloudreve/v3/pkg/hashid"
	"github.com/cloudreve/Cloudreve/v3/pkg/serializer"
	"github.com/cloudreve/Cloudreve/v3/pkg/task"
	"github.com/cloudreve/Cloudreve/v3/pkg/util"
	"github.com/gin-gonic/gin"
)

// ItemMoveService 處理多文件/目錄移動
type ItemMoveService struct {
	SrcDir string        `json:"src_dir" binding:"required,min=1,max=65535"`
	Src    ItemIDService `json:"src"`
	Dst    string        `json:"dst" binding:"required,min=1,max=65535"`
}

// ItemRenameService 處理多文件/目錄重新命名
type ItemRenameService struct {
	Src     ItemIDService `json:"src"`
	NewName string        `json:"new_name" binding:"required,min=1,max=255"`
}

// ItemService 處理多文件/目錄相關服務
type ItemService struct {
	Items []uint `json:"items"`
	Dirs  []uint `json:"dirs"`
}

// ItemIDService 處理多文件/目錄相關服務，欄位值為HashID，可透過Raw()方法獲取原始ID
type ItemIDService struct {
	Items  []string `json:"items"`
	Dirs   []string `json:"dirs"`
	Source *ItemService
}

// ItemCompressService 文件壓縮任務服務
type ItemCompressService struct {
	Src  ItemIDService `json:"src"`
	Dst  string        `json:"dst" binding:"required,min=1,max=65535"`
	Name string        `json:"name" binding:"required,min=1,max=255"`
}

// ItemDecompressService 文件解壓縮任務服務
type ItemDecompressService struct {
	Src string `json:"src"`
	Dst string `json:"dst" binding:"required,min=1,max=65535"`
}

// ItemPropertyService 獲取物件屬性服務
type ItemPropertyService struct {
	ID        string `binding:"required"`
	TraceRoot bool   `form:"trace_root"`
	IsFolder  bool   `form:"is_folder"`
}

// Raw 批次解碼HashID，獲取原始ID
func (service *ItemIDService) Raw() *ItemService {
	if service.Source != nil {
		return service.Source
	}

	service.Source = &ItemService{
		Dirs:  make([]uint, 0, len(service.Dirs)),
		Items: make([]uint, 0, len(service.Items)),
	}
	for _, folder := range service.Dirs {
		id, err := hashid.DecodeHashID(folder, hashid.FolderID)
		if err == nil {
			service.Source.Dirs = append(service.Source.Dirs, id)
		}
	}
	for _, file := range service.Items {
		id, err := hashid.DecodeHashID(file, hashid.FileID)
		if err == nil {
			service.Source.Items = append(service.Source.Items, id)
		}
	}

	return service.Source
}

// CreateDecompressTask 建立文件解壓縮任務
func (service *ItemDecompressService) CreateDecompressTask(c *gin.Context) serializer.Response {
	// 建立文件系統
	fs, err := filesystem.NewFileSystemFromContext(c)
	if err != nil {
		return serializer.Err(serializer.CodePolicyNotAllowed, err.Error(), err)
	}
	defer fs.Recycle()

	// 檢查使用者群組權限
	if !fs.User.Group.OptionsSerialized.ArchiveTask {
		return serializer.Err(serializer.CodeGroupNotAllowed, "目前使用者群組無法進行此操作", nil)
	}

	// 存放目錄是否存在
	if exist, _ := fs.IsPathExist(service.Dst); !exist {
		return serializer.Err(serializer.CodeNotFound, "存放路徑不存在", nil)
	}

	// 壓縮包是否存在
	exist, file := fs.IsFileExist(service.Src)
	if !exist {
		return serializer.Err(serializer.CodeNotFound, "文件不存在", nil)
	}

	// 文件尺寸限制
	if fs.User.Group.OptionsSerialized.DecompressSize != 0 && file.Size > fs.User.Group.
		OptionsSerialized.DecompressSize {
		return serializer.Err(serializer.CodeParamErr, "文件太大", nil)
	}

	// 必須是zip壓縮包
	if !strings.HasSuffix(file.Name, ".zip") {
		return serializer.Err(serializer.CodeParamErr, "只能解壓 ZIP 格式的壓縮文件", nil)
	}

	// 建立任務
	job, err := task.NewDecompressTask(fs.User, service.Src, service.Dst)
	if err != nil {
		return serializer.Err(serializer.CodeNotSet, "任務建立失敗", err)
	}
	task.TaskPoll.Submit(job)

	return serializer.Response{}

}

// CreateCompressTask 建立文件壓縮任務
func (service *ItemCompressService) CreateCompressTask(c *gin.Context) serializer.Response {
	// 建立文件系統
	fs, err := filesystem.NewFileSystemFromContext(c)
	if err != nil {
		return serializer.Err(serializer.CodePolicyNotAllowed, err.Error(), err)
	}
	defer fs.Recycle()

	// 檢查使用者群組權限
	if !fs.User.Group.OptionsSerialized.ArchiveTask {
		return serializer.Err(serializer.CodeGroupNotAllowed, "目前使用者群組無法進行此操作", nil)
	}

	// 補齊壓縮文件副檔名（如果沒有）
	if !strings.HasSuffix(service.Name, ".zip") {
		service.Name += ".zip"
	}

	// 存放目錄是否存在，是否重名
	if exist, _ := fs.IsPathExist(service.Dst); !exist {
		return serializer.Err(serializer.CodeNotFound, "存放路徑不存在", nil)
	}
	if exist, _ := fs.IsFileExist(path.Join(service.Dst, service.Name)); exist {
		return serializer.ParamErr("名為 "+service.Name+" 的文件已存在", nil)
	}

	// 檢查檔案名合法性
	if !fs.ValidateLegalName(context.Background(), service.Name) {
		return serializer.ParamErr("檔案名非法", nil)
	}
	if !fs.ValidateExtension(context.Background(), service.Name) {
		return serializer.ParamErr("不允許儲存此副檔名的文件", nil)
	}

	// 遞迴列出待壓縮子目錄
	folders, err := model.GetRecursiveChildFolder(service.Src.Raw().Dirs, fs.User.ID, true)
	if err != nil {
		return serializer.Err(serializer.CodeDBError, "無法列出子目錄", err)
	}

	// 列出所有待壓縮文件
	files, err := model.GetChildFilesOfFolders(&folders)
	if err != nil {
		return serializer.Err(serializer.CodeDBError, "無法列出子文件", err)
	}

	// 計算待壓縮檔案大小
	var totalSize uint64
	for i := 0; i < len(files); i++ {
		totalSize += files[i].Size
	}

	// 文件尺寸限制
	if fs.User.Group.OptionsSerialized.DecompressSize != 0 && totalSize > fs.User.Group.
		OptionsSerialized.CompressSize {
		return serializer.Err(serializer.CodeParamErr, "文件太大", nil)
	}

	// 按照平均壓縮率計算使用者空間是否足夠
	compressRatio := 0.4
	spaceNeeded := uint64(math.Round(float64(totalSize) * compressRatio))
	if fs.User.GetRemainingCapacity() < spaceNeeded {
		return serializer.Err(serializer.CodeParamErr, "剩餘空間不足", err)
	}

	// 建立任務
	job, err := task.NewCompressTask(fs.User, path.Join(service.Dst, service.Name), service.Src.Raw().Dirs,
		service.Src.Raw().Items)
	if err != nil {
		return serializer.Err(serializer.CodeNotSet, "任務建立失敗", err)
	}
	task.TaskPoll.Submit(job)

	return serializer.Response{}

}

// Archive 建立歸檔
func (service *ItemIDService) Archive(ctx context.Context, c *gin.Context) serializer.Response {
	// 建立文件系統
	fs, err := filesystem.NewFileSystemFromContext(c)
	if err != nil {
		return serializer.Err(serializer.CodePolicyNotAllowed, err.Error(), err)
	}
	defer fs.Recycle()

	// 檢查使用者群組權限
	if !fs.User.Group.OptionsSerialized.ArchiveDownload {
		return serializer.Err(serializer.CodeGroupNotAllowed, "目前使用者群組無法進行此操作", nil)
	}

	// 開始壓縮
	ctx = context.WithValue(ctx, fsctx.GinCtx, c)
	items := service.Raw()
	zipFile, err := fs.Compress(ctx, items.Dirs, items.Items, true)
	if err != nil {
		return serializer.Err(serializer.CodeNotSet, "無法建立壓縮文件", err)
	}

	// 生成一次性壓縮文件下載網址
	siteURL, err := url.Parse(model.GetSettingByName("siteURL"))
	if err != nil {
		return serializer.Err(serializer.CodeNotSet, "無法解析站點URL", err)
	}
	zipID := util.RandStringRunes(16)
	ttl := model.GetIntSetting("archive_timeout", 30)
	signedURI, err := auth.SignURI(
		auth.General,
		fmt.Sprintf("/api/v3/file/archive/%s/archive.zip", zipID),
		time.Now().Unix()+int64(ttl),
	)
	finalURL := siteURL.ResolveReference(signedURI).String()

	// 將壓縮文件記錄存入快取
	err = cache.Set("archive_"+zipID, zipFile, ttl)
	if err != nil {
		return serializer.Err(serializer.CodeIOFailed, "無法寫入快取", err)
	}

	return serializer.Response{
		Code: 0,
		Data: finalURL,
	}
}

// Delete 刪除物件
func (service *ItemIDService) Delete(ctx context.Context, c *gin.Context) serializer.Response {
	// 建立文件系統
	fs, err := filesystem.NewFileSystemFromContext(c)
	if err != nil {
		return serializer.Err(serializer.CodePolicyNotAllowed, err.Error(), err)
	}
	defer fs.Recycle()

	// 刪除物件
	items := service.Raw()
	err = fs.Delete(ctx, items.Dirs, items.Items, false)
	if err != nil {
		return serializer.Err(serializer.CodeNotSet, err.Error(), err)
	}

	return serializer.Response{
		Code: 0,
	}

}

// Move 移動物件
func (service *ItemMoveService) Move(ctx context.Context, c *gin.Context) serializer.Response {
	// 建立文件系統
	fs, err := filesystem.NewFileSystemFromContext(c)
	if err != nil {
		return serializer.Err(serializer.CodePolicyNotAllowed, err.Error(), err)
	}
	defer fs.Recycle()

	// 移動物件
	items := service.Src.Raw()
	err = fs.Move(ctx, items.Dirs, items.Items, service.SrcDir, service.Dst)
	if err != nil {
		return serializer.Err(serializer.CodeNotSet, err.Error(), err)
	}

	return serializer.Response{
		Code: 0,
	}

}

// Copy 複製物件
func (service *ItemMoveService) Copy(ctx context.Context, c *gin.Context) serializer.Response {
	// 複製操作只能對一個目錄或文件物件進行操作
	if len(service.Src.Items)+len(service.Src.Dirs) > 1 {
		return serializer.ParamErr("只能複製一個物件", nil)
	}

	// 建立文件系統
	fs, err := filesystem.NewFileSystemFromContext(c)
	if err != nil {
		return serializer.Err(serializer.CodePolicyNotAllowed, err.Error(), err)
	}
	defer fs.Recycle()

	// 複製物件
	err = fs.Copy(ctx, service.Src.Raw().Dirs, service.Src.Raw().Items, service.SrcDir, service.Dst)
	if err != nil {
		return serializer.Err(serializer.CodeNotSet, err.Error(), err)
	}

	return serializer.Response{
		Code: 0,
	}

}

// Rename 重新命名物件
func (service *ItemRenameService) Rename(ctx context.Context, c *gin.Context) serializer.Response {
	// 重新命名作只能對一個目錄或文件物件進行操作
	if len(service.Src.Items)+len(service.Src.Dirs) > 1 {
		return serializer.ParamErr("只能操作一個物件", nil)
	}

	// 建立文件系統
	fs, err := filesystem.NewFileSystemFromContext(c)
	if err != nil {
		return serializer.Err(serializer.CodePolicyNotAllowed, err.Error(), err)
	}
	defer fs.Recycle()

	// 重新命名物件
	err = fs.Rename(ctx, service.Src.Raw().Dirs, service.Src.Raw().Items, service.NewName)
	if err != nil {
		return serializer.Err(serializer.CodeNotSet, err.Error(), err)
	}

	return serializer.Response{
		Code: 0,
	}
}

// GetProperty 獲取物件的屬性
func (service *ItemPropertyService) GetProperty(ctx context.Context, c *gin.Context) serializer.Response {
	userCtx, _ := c.Get("user")
	user := userCtx.(*model.User)

	var props serializer.ObjectProps
	props.QueryDate = time.Now()

	// 如果是文件物件
	if !service.IsFolder {
		res, err := hashid.DecodeHashID(service.ID, hashid.FileID)
		if err != nil {
			return serializer.Err(serializer.CodeNotFound, "物件不存在", err)
		}

		file, err := model.GetFilesByIDs([]uint{res}, user.ID)
		if err != nil {
			return serializer.DBErr("找不到文件", err)
		}

		props.CreatedAt = file[0].CreatedAt
		props.UpdatedAt = file[0].UpdatedAt
		props.Policy = file[0].GetPolicy().Name
		props.Size = file[0].Size

		// 尋找父目錄
		if service.TraceRoot {
			parent, err := model.GetFoldersByIDs([]uint{file[0].FolderID}, user.ID)
			if err != nil {
				return serializer.DBErr("找不到父目錄", err)
			}

			if err := parent[0].TraceRoot(); err != nil {
				return serializer.DBErr("無法溯源父目錄", err)
			}

			props.Path = path.Join(parent[0].Position, parent[0].Name)
		}
	} else {
		res, err := hashid.DecodeHashID(service.ID, hashid.FolderID)
		if err != nil {
			return serializer.Err(serializer.CodeNotFound, "物件不存在", err)
		}

		// 如果物件是目錄, 先嘗試返回快取結果
		if cacheRes, ok := cache.Get(fmt.Sprintf("folder_props_%d", res)); ok {
			return serializer.Response{Data: cacheRes.(serializer.ObjectProps)}
		}

		folder, err := model.GetFoldersByIDs([]uint{res}, user.ID)
		if err != nil {
			return serializer.DBErr("找不到目錄", err)
		}

		props.CreatedAt = folder[0].CreatedAt
		props.UpdatedAt = folder[0].UpdatedAt

		// 統計子目錄
		childFolders, err := model.GetRecursiveChildFolder([]uint{folder[0].ID},
			user.ID, true)
		if err != nil {
			return serializer.DBErr("無法列取子目錄", err)
		}
		props.ChildFolderNum = len(childFolders) - 1

		// 統計子文件
		files, err := model.GetChildFilesOfFolders(&childFolders)
		if err != nil {
			return serializer.DBErr("無法列取子文件", err)
		}

		// 統計子文件個數和大小
		props.ChildFileNum = len(files)
		for i := 0; i < len(files); i++ {
			props.Size += files[i].Size
		}

		// 尋找父目錄
		if service.TraceRoot {
			if err := folder[0].TraceRoot(); err != nil {
				return serializer.DBErr("無法溯源父目錄", err)
			}

			props.Path = folder[0].Position
		}

		// 如果列取物件是目錄，則快取結果
		cache.Set(fmt.Sprintf("folder_props_%d", res), props,
			model.GetIntSetting("folder_props_timeout", 300))
	}

	return serializer.Response{
		Code: 0,
		Data: props,
	}
}
