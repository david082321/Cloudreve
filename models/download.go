package model

import (
	"encoding/json"

	"github.com/cloudreve/Cloudreve/v3/pkg/aria2/rpc"
	"github.com/cloudreve/Cloudreve/v3/pkg/util"
	"github.com/jinzhu/gorm"
)

// Download 離線下載佇列模型
type Download struct {
	gorm.Model
	Status         int    // 任務狀態
	Type           int    // 任務類型
	Source         string `gorm:"type:text"` // 文件下載網址
	TotalSize      uint64 // 檔案大小
	DownloadedSize uint64 // 檔案大小
	GID            string `gorm:"size:32,index:gid"` // 任務ID
	Speed          int    // 下載速度
	Parent         string `gorm:"type:text"`       // 儲存目錄
	Attrs          string `gorm:"size:4294967295"` // 任務狀態屬性
	Error          string `gorm:"type:text"`       // 錯誤描述
	Dst            string `gorm:"type:text"`       // 使用者文件系統儲存父目錄路徑
	UserID         uint   // 發起者UID
	TaskID         uint   // 對應的轉存任務ID

	// 關聯模型
	User *User `gorm:"PRELOAD:false,association_autoupdate:false"`

	// 資料庫忽略欄位
	StatusInfo rpc.StatusInfo `gorm:"-"`
	Task       *Task          `gorm:"-"`
}

// AfterFind 找到下載任務後的鉤子，處理Status結構
func (task *Download) AfterFind() (err error) {
	// 解析狀態
	if task.Attrs != "" {
		err = json.Unmarshal([]byte(task.Attrs), &task.StatusInfo)
	}

	if task.TaskID != 0 {
		task.Task, _ = GetTasksByID(task.TaskID)
	}

	return err
}

// BeforeSave Save下載任務前的鉤子
func (task *Download) BeforeSave() (err error) {
	// 解析狀態
	if task.Attrs != "" {
		err = json.Unmarshal([]byte(task.Attrs), &task.StatusInfo)
	}
	return err
}

// Create 建立離線下載記錄
func (task *Download) Create() (uint, error) {
	if err := DB.Create(task).Error; err != nil {
		util.Log().Warning("無法插入離線下載記錄, %s", err)
		return 0, err
	}
	return task.ID, nil
}

// Save 更新
func (task *Download) Save() error {
	if err := DB.Save(task).Error; err != nil {
		util.Log().Warning("無法更新離線下載記錄, %s", err)
		return err
	}
	return nil
}

// GetDownloadsByStatus 根據狀態檢索下載
func GetDownloadsByStatus(status ...int) []Download {
	var tasks []Download
	DB.Where("status in (?)", status).Find(&tasks)
	return tasks
}

// GetDownloadsByStatusAndUser 根據狀態檢索和使用者ID下載
// page 為 0 表示列出所有，非零時分頁
func GetDownloadsByStatusAndUser(page, uid uint, status ...int) []Download {
	var tasks []Download
	dbChain := DB
	if page > 0 {
		dbChain = dbChain.Limit(10).Offset((page - 1) * 10).Order("updated_at DESC")
	}
	dbChain.Where("user_id = ? and status in (?)", uid, status).Find(&tasks)
	return tasks
}

// GetDownloadByGid 根據GID和使用者ID尋找下載
func GetDownloadByGid(gid string, uid uint) (*Download, error) {
	download := &Download{}
	result := DB.Where("user_id = ? and g_id = ?", uid, gid).First(download)
	return download, result.Error
}

// GetOwner 獲取下載任務所屬使用者
func (task *Download) GetOwner() *User {
	if task.User == nil {
		if user, err := GetUserByID(task.UserID); err == nil {
			return &user
		}
	}
	return task.User
}

// Delete 刪除離線下載記錄
func (download *Download) Delete() error {
	return DB.Model(download).Delete(download).Error
}
