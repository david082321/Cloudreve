package model

import (
	"errors"
	"path"
	"time"

	"github.com/cloudreve/Cloudreve/v3/pkg/util"
	"github.com/jinzhu/gorm"
)

// Folder 目錄
type Folder struct {
	// 表欄位
	gorm.Model
	Name     string `gorm:"unique_index:idx_only_one_name"`
	ParentID *uint  `gorm:"index:parent_id;unique_index:idx_only_one_name"`
	OwnerID  uint   `gorm:"index:owner_id"`

	// 資料庫忽略欄位
	Position string `gorm:"-"`
}

// Create 建立目錄
func (folder *Folder) Create() (uint, error) {
	if err := DB.Create(folder).Error; err != nil {
		util.Log().Warning("無法插入目錄記錄, %s", err)
		return 0, err
	}
	return folder.ID, nil
}

// GetChild 返回folder下名為name的子目錄，不存在則返回錯誤
func (folder *Folder) GetChild(name string) (*Folder, error) {
	var resFolder Folder
	err := DB.
		Where("parent_id = ? AND owner_id = ? AND name = ?", folder.ID, folder.OwnerID, name).
		First(&resFolder).Error

	// 將子目錄的路徑傳遞下去
	if err == nil {
		resFolder.Position = path.Join(folder.Position, folder.Name)
	}
	return &resFolder, err
}

// TraceRoot 向上遞迴尋找父目錄
func (folder *Folder) TraceRoot() error {
	if folder.ParentID == nil {
		return nil
	}

	var parentFolder Folder
	err := DB.
		Where("id = ? AND owner_id = ?", folder.ParentID, folder.OwnerID).
		First(&parentFolder).Error

	if err == nil {
		err := parentFolder.TraceRoot()
		folder.Position = path.Join(parentFolder.Position, parentFolder.Name)
		return err
	}

	return err
}

// GetChildFolder 尋找子目錄
func (folder *Folder) GetChildFolder() ([]Folder, error) {
	var folders []Folder
	result := DB.Where("parent_id = ?", folder.ID).Find(&folders)

	if result.Error == nil {
		for i := 0; i < len(folders); i++ {
			folders[i].Position = path.Join(folder.Position, folder.Name)
		}
	}
	return folders, result.Error
}

// GetRecursiveChildFolder 尋找所有遞迴子目錄，包括自身
func GetRecursiveChildFolder(dirs []uint, uid uint, includeSelf bool) ([]Folder, error) {
	folders := make([]Folder, 0, len(dirs))
	var err error

	var parFolders []Folder
	result := DB.Where("owner_id = ? and id in (?)", uid, dirs).Find(&parFolders)
	if result.Error != nil {
		return folders, err
	}

	// 整理父目錄的ID
	var parentIDs = make([]uint, 0, len(parFolders))
	for _, folder := range parFolders {
		parentIDs = append(parentIDs, folder.ID)
	}

	if includeSelf {
		// 合併至最終結果
		folders = append(folders, parFolders...)
	}
	parFolders = []Folder{}

	// 遞迴查詢子目錄,最大遞迴65535次
	for i := 0; i < 65535; i++ {

		result = DB.Where("owner_id = ? and parent_id in (?)", uid, parentIDs).Find(&parFolders)

		// 查詢結束條件
		if len(parFolders) == 0 {
			break
		}

		// 整理父目錄的ID
		parentIDs = make([]uint, 0, len(parFolders))
		for _, folder := range parFolders {
			parentIDs = append(parentIDs, folder.ID)
		}

		// 合併至最終結果
		folders = append(folders, parFolders...)
		parFolders = []Folder{}

	}

	return folders, err
}

// DeleteFolderByIDs 根據給定ID批次刪除目錄記錄
func DeleteFolderByIDs(ids []uint) error {
	result := DB.Where("id in (?)", ids).Unscoped().Delete(&Folder{})
	return result.Error
}

// GetFoldersByIDs 根據ID和使用者尋找所有目錄
func GetFoldersByIDs(ids []uint, uid uint) ([]Folder, error) {
	var folders []Folder
	result := DB.Where("id in (?) AND owner_id = ?", ids, uid).Find(&folders)
	return folders, result.Error
}

// MoveOrCopyFileTo 將此目錄下的files移動或複製至dstFolder，
// 返回此操作新增的容量
func (folder *Folder) MoveOrCopyFileTo(files []uint, dstFolder *Folder, isCopy bool) (uint64, error) {
	// 已複製文件的總大小
	var copiedSize uint64

	if isCopy {
		// 檢索出要複製的文件
		var originFiles = make([]File, 0, len(files))
		if err := DB.Where(
			"id in (?) and user_id = ? and folder_id = ?",
			files,
			folder.OwnerID,
			folder.ID,
		).Find(&originFiles).Error; err != nil {
			return 0, err
		}

		// 複製文件記錄
		for _, oldFile := range originFiles {
			oldFile.Model = gorm.Model{}
			oldFile.FolderID = dstFolder.ID
			oldFile.UserID = dstFolder.OwnerID

			if err := DB.Create(&oldFile).Error; err != nil {
				return copiedSize, err
			}

			copiedSize += oldFile.Size
		}

	} else {
		// 更改頂級要移動文件的父目錄指向
		err := DB.Model(File{}).Where(
			"id in (?) and user_id = ? and folder_id = ?",
			files,
			folder.OwnerID,
			folder.ID,
		).
			Update(map[string]interface{}{
				"folder_id": dstFolder.ID,
			}).
			Error
		if err != nil {
			return 0, err
		}

	}

	return copiedSize, nil

}

// CopyFolderTo 將此目錄及其子目錄及文件遞迴複製至dstFolder
// 返回此操作新增的容量
func (folder *Folder) CopyFolderTo(folderID uint, dstFolder *Folder) (size uint64, err error) {
	// 列出所有子目錄
	subFolders, err := GetRecursiveChildFolder([]uint{folderID}, folder.OwnerID, true)
	if err != nil {
		return 0, err
	}

	// 抽離所有子目錄的ID
	var subFolderIDs = make([]uint, len(subFolders))
	for key, value := range subFolders {
		subFolderIDs[key] = value.ID
	}

	// 複製子目錄
	var newIDCache = make(map[uint]uint)
	for _, folder := range subFolders {
		// 新的父目錄指向
		var newID uint
		// 頂級目錄直接指向新的目的目錄
		if folder.ID == folderID {
			newID = dstFolder.ID
		} else if IDCache, ok := newIDCache[*folder.ParentID]; ok {
			newID = IDCache
		} else {
			util.Log().Warning("無法取得新的父目錄:%d", folder.ParentID)
			return size, errors.New("無法取得新的父目錄")
		}

		// 插入新的目錄記錄
		oldID := folder.ID
		folder.Model = gorm.Model{}
		folder.ParentID = &newID
		folder.OwnerID = dstFolder.OwnerID
		if err = DB.Create(&folder).Error; err != nil {
			return size, err
		}
		// 記錄新的ID以便其子目錄使用
		newIDCache[oldID] = folder.ID

	}

	// 複製文件
	var originFiles = make([]File, 0, len(subFolderIDs))
	if err := DB.Where(
		"user_id = ? and folder_id in (?)",
		folder.OwnerID,
		subFolderIDs,
	).Find(&originFiles).Error; err != nil {
		return 0, err
	}

	// 複製文件記錄
	for _, oldFile := range originFiles {
		oldFile.Model = gorm.Model{}
		oldFile.FolderID = newIDCache[oldFile.FolderID]
		oldFile.UserID = dstFolder.OwnerID
		if err := DB.Create(&oldFile).Error; err != nil {
			return size, err
		}

		size += oldFile.Size
	}

	return size, nil

}

// MoveFolderTo 將folder目錄下的dirs子目錄複製或移動到dstFolder，
// 返回此過程中增加的容量
func (folder *Folder) MoveFolderTo(dirs []uint, dstFolder *Folder) error {
	// 更改頂級要移動目錄的父目錄指向
	err := DB.Model(Folder{}).Where(
		"id in (?) and owner_id = ? and parent_id = ?",
		dirs,
		folder.OwnerID,
		folder.ID,
	).Update(map[string]interface{}{
		"parent_id": dstFolder.ID,
	}).Error

	return err

}

// Rename 重新命名目錄
func (folder *Folder) Rename(new string) error {
	if err := DB.Model(&folder).Update("name", new).Error; err != nil {
		return err
	}
	return nil
}

/*
	實現 FileInfo.FileInfo 介面
	TODO 測試
*/

func (folder *Folder) GetName() string {
	return folder.Name
}

func (folder *Folder) GetSize() uint64 {
	return 0
}
func (folder *Folder) ModTime() time.Time {
	return folder.UpdatedAt
}
func (folder *Folder) IsDir() bool {
	return true
}
func (folder *Folder) GetPosition() string {
	return folder.Position
}
