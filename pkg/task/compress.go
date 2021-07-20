package task

import (
	"context"
	"encoding/json"
	"os"

	model "github.com/cloudreve/Cloudreve/v3/models"
	"github.com/cloudreve/Cloudreve/v3/pkg/filesystem"
	"github.com/cloudreve/Cloudreve/v3/pkg/util"
)

// CompressTask 文件壓縮任務
type CompressTask struct {
	User      *model.User
	TaskModel *model.Task
	TaskProps CompressProps
	Err       *JobError

	zipPath string
}

// CompressProps 壓縮任務屬性
type CompressProps struct {
	Dirs  []uint `json:"dirs"`
	Files []uint `json:"files"`
	Dst   string `json:"dst"`
}

// Props 獲取任務屬性
func (job *CompressTask) Props() string {
	res, _ := json.Marshal(job.TaskProps)
	return string(res)
}

// Type 獲取任務狀態
func (job *CompressTask) Type() int {
	return CompressTaskType
}

// Creator 獲取建立者ID
func (job *CompressTask) Creator() uint {
	return job.User.ID
}

// Model 獲取任務的資料庫模型
func (job *CompressTask) Model() *model.Task {
	return job.TaskModel
}

// SetStatus 設定狀態
func (job *CompressTask) SetStatus(status int) {
	job.TaskModel.SetStatus(status)
}

// SetError 設定任務失敗訊息
func (job *CompressTask) SetError(err *JobError) {
	job.Err = err
	res, _ := json.Marshal(job.Err)
	job.TaskModel.SetError(string(res))

	// 刪除壓縮文件
	job.removeZipFile()
}

func (job *CompressTask) removeZipFile() {
	if job.zipPath != "" {
		if err := os.Remove(job.zipPath); err != nil {
			util.Log().Warning("無法刪除臨時壓縮文件 %s , %s", job.zipPath, err)
		}
	}
}

// SetErrorMsg 設定任務失敗訊息
func (job *CompressTask) SetErrorMsg(msg string) {
	job.SetError(&JobError{Msg: msg})
}

// GetError 返回任務失敗訊息
func (job *CompressTask) GetError() *JobError {
	return job.Err
}

// Do 開始執行任務
func (job *CompressTask) Do() {
	// 建立文件系統
	fs, err := filesystem.NewFileSystem(job.User)
	if err != nil {
		job.SetErrorMsg(err.Error())
		return
	}

	util.Log().Debug("開始壓縮文件")
	job.TaskModel.SetProgress(CompressingProgress)

	// 開始壓縮
	ctx := context.Background()
	zipFile, err := fs.Compress(ctx, job.TaskProps.Dirs, job.TaskProps.Files, false)
	if err != nil {
		job.SetErrorMsg(err.Error())
		return
	}
	job.zipPath = zipFile

	util.Log().Debug("壓縮文件存放至%s，開始上傳", zipFile)
	job.TaskModel.SetProgress(TransferringProgress)

	// 上傳文件
	err = fs.UploadFromPath(ctx, zipFile, job.TaskProps.Dst)
	if err != nil {
		job.SetErrorMsg(err.Error())
		return
	}

	job.removeZipFile()
}

// NewCompressTask 建立壓縮任務
func NewCompressTask(user *model.User, dst string, dirs, files []uint) (Job, error) {
	newTask := &CompressTask{
		User: user,
		TaskProps: CompressProps{
			Dirs:  dirs,
			Files: files,
			Dst:   dst,
		},
	}

	record, err := Record(newTask)
	if err != nil {
		return nil, err
	}
	newTask.TaskModel = record

	return newTask, nil
}

// NewCompressTaskFromModel 從資料庫記錄中復原壓縮任務
func NewCompressTaskFromModel(task *model.Task) (Job, error) {
	user, err := model.GetActiveUserByID(task.UserID)
	if err != nil {
		return nil, err
	}
	newTask := &CompressTask{
		User:      &user,
		TaskModel: task,
	}

	err = json.Unmarshal([]byte(task.Props), &newTask.TaskProps)
	if err != nil {
		return nil, err
	}

	return newTask, nil
}
