package model

import (
	"github.com/cloudreve/Cloudreve/v3/pkg/util"
	"github.com/jinzhu/gorm"
)

// Task 任務模型
type Task struct {
	gorm.Model
	Status   int    // 任務狀態
	Type     int    // 任務類型
	UserID   uint   // 發起者UID，0表示為系統發起
	Progress int    // 進度
	Error    string `gorm:"type:text"` // 錯誤訊息
	Props    string `gorm:"type:text"` // 任務屬性
}

// Create 建立任務記錄
func (task *Task) Create() (uint, error) {
	if err := DB.Create(task).Error; err != nil {
		util.Log().Warning("無法插入任務記錄, %s", err)
		return 0, err
	}
	return task.ID, nil
}

// SetStatus 設定任務狀態
func (task *Task) SetStatus(status int) error {
	return DB.Model(task).Select("status").Updates(map[string]interface{}{"status": status}).Error
}

// SetProgress 設定任務進度
func (task *Task) SetProgress(progress int) error {
	return DB.Model(task).Select("progress").Updates(map[string]interface{}{"progress": progress}).Error
}

// SetError 設定錯誤訊息
func (task *Task) SetError(err string) error {
	return DB.Model(task).Select("error").Updates(map[string]interface{}{"error": err}).Error
}

// GetTasksByStatus 根據狀態檢索任務
func GetTasksByStatus(status ...int) []Task {
	var tasks []Task
	DB.Where("status in (?)", status).Find(&tasks)
	return tasks
}

// GetTasksByID 根據ID檢索任務
func GetTasksByID(id interface{}) (*Task, error) {
	task := &Task{}
	result := DB.Where("id = ?", id).First(task)
	return task, result.Error
}

// ListTasks 列出使用者所屬的任務
func ListTasks(uid uint, page, pageSize int, order string) ([]Task, int) {
	var (
		tasks []Task
		total int
	)
	dbChain := DB
	dbChain = dbChain.Where("user_id = ?", uid)

	// 計算總數用於分頁
	dbChain.Model(&Share{}).Count(&total)

	// 查詢記錄
	dbChain.Limit(pageSize).Offset((page - 1) * pageSize).Order(order).Find(&tasks)

	return tasks, total
}
