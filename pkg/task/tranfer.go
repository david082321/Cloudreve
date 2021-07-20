package task

import (
	"context"
	"encoding/json"
	"os"
	"path"
	"path/filepath"
	"strings"

	model "github.com/cloudreve/Cloudreve/v3/models"
	"github.com/cloudreve/Cloudreve/v3/pkg/filesystem"
	"github.com/cloudreve/Cloudreve/v3/pkg/filesystem/fsctx"
	"github.com/cloudreve/Cloudreve/v3/pkg/util"
)

// TransferTask 文件中轉任務
type TransferTask struct {
	User      *model.User
	TaskModel *model.Task
	TaskProps TransferProps
	Err       *JobError

	zipPath string
}

// TransferProps 中轉任務屬性
type TransferProps struct {
	Src    []string `json:"src"`    // 原始文件
	Parent string   `json:"parent"` // 父目錄
	Dst    string   `json:"dst"`    // 目的目錄ID
	// 將會保留原始文件的目錄結構，Src 除去 Parent 開頭作為最終路徑
	TrimPath bool `json:"trim_path"`
}

// Props 獲取任務屬性
func (job *TransferTask) Props() string {
	res, _ := json.Marshal(job.TaskProps)
	return string(res)
}

// Type 獲取任務狀態
func (job *TransferTask) Type() int {
	return TransferTaskType
}

// Creator 獲取建立者ID
func (job *TransferTask) Creator() uint {
	return job.User.ID
}

// Model 獲取任務的資料庫模型
func (job *TransferTask) Model() *model.Task {
	return job.TaskModel
}

// SetStatus 設定狀態
func (job *TransferTask) SetStatus(status int) {
	job.TaskModel.SetStatus(status)
}

// SetError 設定任務失敗訊息
func (job *TransferTask) SetError(err *JobError) {
	job.Err = err
	res, _ := json.Marshal(job.Err)
	job.TaskModel.SetError(string(res))

}

// SetErrorMsg 設定任務失敗訊息
func (job *TransferTask) SetErrorMsg(msg string, err error) {
	jobErr := &JobError{Msg: msg}
	if err != nil {
		jobErr.Error = err.Error()
	}
	job.SetError(jobErr)
}

// GetError 返回任務失敗訊息
func (job *TransferTask) GetError() *JobError {
	return job.Err
}

// Do 開始執行任務
func (job *TransferTask) Do() {
	defer job.Recycle()

	// 建立文件系統
	fs, err := filesystem.NewFileSystem(job.User)
	if err != nil {
		job.SetErrorMsg(err.Error(), nil)
		return
	}

	for index, file := range job.TaskProps.Src {
		job.TaskModel.SetProgress(index)

		dst := path.Join(job.TaskProps.Dst, filepath.Base(file))
		if job.TaskProps.TrimPath {
			// 保留原始目錄
			trim := util.FormSlash(job.TaskProps.Parent)
			src := util.FormSlash(file)
			dst = path.Join(job.TaskProps.Dst, strings.TrimPrefix(src, trim))
		}

		ctx := context.WithValue(context.Background(), fsctx.DisableOverwrite, true)
		err = fs.UploadFromPath(ctx, file, dst)
		if err != nil {
			job.SetErrorMsg("文件轉存失敗", err)
		}
	}

}

// Recycle 回收暫存檔
func (job *TransferTask) Recycle() {
	err := os.RemoveAll(job.TaskProps.Parent)
	if err != nil {
		util.Log().Warning("無法刪除中轉暫存資料夾[%s], %s", job.TaskProps.Parent, err)
	}

}

// NewTransferTask 建立中轉任務
func NewTransferTask(user uint, src []string, dst, parent string, trim bool) (Job, error) {
	creator, err := model.GetActiveUserByID(user)
	if err != nil {
		return nil, err
	}

	newTask := &TransferTask{
		User: &creator,
		TaskProps: TransferProps{
			Src:      src,
			Parent:   parent,
			Dst:      dst,
			TrimPath: trim,
		},
	}

	record, err := Record(newTask)
	if err != nil {
		return nil, err
	}
	newTask.TaskModel = record

	return newTask, nil
}

// NewTransferTaskFromModel 從資料庫記錄中復原中轉任務
func NewTransferTaskFromModel(task *model.Task) (Job, error) {
	user, err := model.GetActiveUserByID(task.UserID)
	if err != nil {
		return nil, err
	}
	newTask := &TransferTask{
		User:      &user,
		TaskModel: task,
	}

	err = json.Unmarshal([]byte(task.Props), &newTask.TaskProps)
	if err != nil {
		return nil, err
	}

	return newTask, nil
}
