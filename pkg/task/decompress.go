package task

import (
	"context"
	"encoding/json"

	model "github.com/cloudreve/Cloudreve/v3/models"
	"github.com/cloudreve/Cloudreve/v3/pkg/filesystem"
	"github.com/cloudreve/Cloudreve/v3/pkg/filesystem/fsctx"
)

// DecompressTask 文件壓縮任務
type DecompressTask struct {
	User      *model.User
	TaskModel *model.Task
	TaskProps DecompressProps
	Err       *JobError

	zipPath string
}

// DecompressProps 壓縮任務屬性
type DecompressProps struct {
	Src string `json:"src"`
	Dst string `json:"dst"`
}

// Props 獲取任務屬性
func (job *DecompressTask) Props() string {
	res, _ := json.Marshal(job.TaskProps)
	return string(res)
}

// Type 獲取任務狀態
func (job *DecompressTask) Type() int {
	return DecompressTaskType
}

// Creator 獲取建立者ID
func (job *DecompressTask) Creator() uint {
	return job.User.ID
}

// Model 獲取任務的資料庫模型
func (job *DecompressTask) Model() *model.Task {
	return job.TaskModel
}

// SetStatus 設定狀態
func (job *DecompressTask) SetStatus(status int) {
	job.TaskModel.SetStatus(status)
}

// SetError 設定任務失敗訊息
func (job *DecompressTask) SetError(err *JobError) {
	job.Err = err
	res, _ := json.Marshal(job.Err)
	job.TaskModel.SetError(string(res))
}

// SetErrorMsg 設定任務失敗訊息
func (job *DecompressTask) SetErrorMsg(msg string, err error) {
	jobErr := &JobError{Msg: msg}
	if err != nil {
		jobErr.Error = err.Error()
	}
	job.SetError(jobErr)
}

// GetError 返回任務失敗訊息
func (job *DecompressTask) GetError() *JobError {
	return job.Err
}

// Do 開始執行任務
func (job *DecompressTask) Do() {
	// 建立文件系統
	fs, err := filesystem.NewFileSystem(job.User)
	if err != nil {
		job.SetErrorMsg("無法建立文件系統", err)
		return
	}

	job.TaskModel.SetProgress(DecompressingProgress)

	// 禁止重名覆蓋
	ctx := context.Background()
	ctx = context.WithValue(ctx, fsctx.DisableOverwrite, true)

	err = fs.Decompress(ctx, job.TaskProps.Src, job.TaskProps.Dst)
	if err != nil {
		job.SetErrorMsg("解壓縮失敗", err)
		return
	}

}

// NewDecompressTask 建立壓縮任務
func NewDecompressTask(user *model.User, src, dst string) (Job, error) {
	newTask := &DecompressTask{
		User: user,
		TaskProps: DecompressProps{
			Src: src,
			Dst: dst,
		},
	}

	record, err := Record(newTask)
	if err != nil {
		return nil, err
	}
	newTask.TaskModel = record

	return newTask, nil
}

// NewDecompressTaskFromModel 從資料庫記錄中復原壓縮任務
func NewDecompressTaskFromModel(task *model.Task) (Job, error) {
	user, err := model.GetActiveUserByID(task.UserID)
	if err != nil {
		return nil, err
	}
	newTask := &DecompressTask{
		User:      &user,
		TaskModel: task,
	}

	err = json.Unmarshal([]byte(task.Props), &newTask.TaskProps)
	if err != nil {
		return nil, err
	}

	return newTask, nil
}
