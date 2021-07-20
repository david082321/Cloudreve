package task

import (
	model "github.com/cloudreve/Cloudreve/v3/models"
	"github.com/cloudreve/Cloudreve/v3/pkg/util"
)

// 任務類型
const (
	// CompressTaskType 壓縮任務
	CompressTaskType = iota
	// DecompressTaskType 解壓縮任務
	DecompressTaskType
	// TransferTaskType 中轉任務
	TransferTaskType
	// ImportTaskType 匯入任務
	ImportTaskType
)

// 任務狀態
const (
	// Queued 排隊中
	Queued = iota
	// Processing 處理中
	Processing
	// Error 失敗
	Error
	// Canceled 取消
	Canceled
	// Complete 完成
	Complete
)

// 任務進度
const (
	// PendingProgress 等待中
	PendingProgress = iota
	// Compressing 壓縮中
	CompressingProgress
	// Decompressing 解壓縮中
	DecompressingProgress
	// Downloading 下載中
	DownloadingProgress
	// Transferring 轉存中
	TransferringProgress
	// ListingProgress 索引中
	ListingProgress
	// InsertingProgress 插入中
	InsertingProgress
)

// Job 任務介面
type Job interface {
	Type() int           // 返回任務類型
	Creator() uint       // 返回建立者ID
	Props() string       // 返回序列化後的任務屬性
	Model() *model.Task  // 返回對應的資料庫模型
	SetStatus(int)       // 設定任務狀態
	Do()                 // 開始執行任務
	SetError(*JobError)  // 設定任務失敗訊息
	GetError() *JobError // 獲取任務執行結果，返回nil表示成功完成執行
}

// JobError 任務失敗訊息
type JobError struct {
	Msg   string `json:"msg,omitempty"`
	Error string `json:"error,omitempty"`
}

// Record 將任務記錄到資料庫中
func Record(job Job) (*model.Task, error) {
	record := model.Task{
		Status:   Queued,
		Type:     job.Type(),
		UserID:   job.Creator(),
		Progress: 0,
		Error:    "",
		Props:    job.Props(),
	}
	_, err := record.Create()
	return &record, err
}

// Resume 從資料庫中復原未完成任務
func Resume() {
	tasks := model.GetTasksByStatus(Queued, Processing)
	if len(tasks) == 0 {
		return
	}
	util.Log().Info("從資料庫中復原 %d 個未完成任務", len(tasks))

	for i := 0; i < len(tasks); i++ {
		job, err := GetJobFromModel(&tasks[i])
		if err != nil {
			util.Log().Warning("無法復原任務，%s", err)
			continue
		}

		TaskPoll.Submit(job)
	}
}

// GetJobFromModel 從資料庫給定模型獲取任務
func GetJobFromModel(task *model.Task) (Job, error) {
	switch task.Type {
	case CompressTaskType:
		return NewCompressTaskFromModel(task)
	case DecompressTaskType:
		return NewDecompressTaskFromModel(task)
	case TransferTaskType:
		return NewTransferTaskFromModel(task)
	case ImportTaskType:
		return NewImportTaskFromModel(task)
	default:
		return nil, ErrUnknownTaskType
	}
}
