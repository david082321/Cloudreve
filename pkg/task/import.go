package task

import (
	"context"
	"encoding/json"
	"path"

	model "github.com/cloudreve/Cloudreve/v3/models"
	"github.com/cloudreve/Cloudreve/v3/pkg/filesystem"
	"github.com/cloudreve/Cloudreve/v3/pkg/filesystem/driver/local"
	"github.com/cloudreve/Cloudreve/v3/pkg/filesystem/fsctx"
	"github.com/cloudreve/Cloudreve/v3/pkg/util"
)

// ImportTask 匯入務
type ImportTask struct {
	User      *model.User
	TaskModel *model.Task
	TaskProps ImportProps
	Err       *JobError
}

// ImportProps 匯入任務屬性
type ImportProps struct {
	PolicyID  uint   `json:"policy_id"`    // 儲存策略ID
	Src       string `json:"src"`          // 原始路徑
	Recursive bool   `json:"is_recursive"` // 是否遞迴匯入
	Dst       string `json:"dst"`          // 目的目錄
}

// Props 獲取任務屬性
func (job *ImportTask) Props() string {
	res, _ := json.Marshal(job.TaskProps)
	return string(res)
}

// Type 獲取任務狀態
func (job *ImportTask) Type() int {
	return ImportTaskType
}

// Creator 獲取建立者ID
func (job *ImportTask) Creator() uint {
	return job.User.ID
}

// Model 獲取任務的資料庫模型
func (job *ImportTask) Model() *model.Task {
	return job.TaskModel
}

// SetStatus 設定狀態
func (job *ImportTask) SetStatus(status int) {
	job.TaskModel.SetStatus(status)
}

// SetError 設定任務失敗訊息
func (job *ImportTask) SetError(err *JobError) {
	job.Err = err
	res, _ := json.Marshal(job.Err)
	job.TaskModel.SetError(string(res))
}

// SetErrorMsg 設定任務失敗訊息
func (job *ImportTask) SetErrorMsg(msg string, err error) {
	jobErr := &JobError{Msg: msg}
	if err != nil {
		jobErr.Error = err.Error()
	}
	job.SetError(jobErr)
}

// GetError 返回任務失敗訊息
func (job *ImportTask) GetError() *JobError {
	return job.Err
}

// Do 開始執行任務
func (job *ImportTask) Do() {
	ctx := context.Background()

	// 尋找儲存策略
	policy, err := model.GetPolicyByID(job.TaskProps.PolicyID)
	if err != nil {
		job.SetErrorMsg("找不到儲存策略", err)
		return
	}

	// 建立文件系統
	job.User.Policy = policy
	fs, err := filesystem.NewFileSystem(job.User)
	if err != nil {
		job.SetErrorMsg(err.Error(), nil)
		return
	}
	defer fs.Recycle()

	// 註冊鉤子
	fs.Use("BeforeAddFile", filesystem.HookValidateFile)
	fs.Use("BeforeAddFile", filesystem.HookValidateCapacity)
	fs.Use("AfterValidateFailed", filesystem.HookGiveBackCapacity)

	// 列取目錄、物件
	job.TaskModel.SetProgress(ListingProgress)
	coxIgnoreConflict := context.WithValue(context.Background(), fsctx.IgnoreDirectoryConflictCtx,
		true)
	objects, err := fs.Handler.List(ctx, job.TaskProps.Src, job.TaskProps.Recursive)
	if err != nil {
		job.SetErrorMsg("無法列取文件", err)
		return
	}

	job.TaskModel.SetProgress(InsertingProgress)

	// 虛擬目錄路徑與folder物件ID的對應
	pathCache := make(map[string]*model.Folder, len(objects))

	// 插入目錄記錄到使用者文件系統
	for _, object := range objects {
		if object.IsDir {
			// 建立目錄
			virtualPath := path.Join(job.TaskProps.Dst, object.RelativePath)
			folder, err := fs.CreateDirectory(coxIgnoreConflict, virtualPath)
			if err != nil {
				util.Log().Warning("匯入任務無法建立使用者目錄[%s], %s", virtualPath, err)
			} else if folder.ID > 0 {
				pathCache[virtualPath] = folder
			}
		}
	}

	// 插入文件記錄到使用者文件系統
	for _, object := range objects {
		if !object.IsDir {
			// 建立文件訊息
			virtualPath := path.Dir(path.Join(job.TaskProps.Dst, object.RelativePath))
			fileHeader := local.FileStream{
				Size:        object.Size,
				VirtualPath: virtualPath,
				Name:        object.Name,
			}
			addFileCtx := context.WithValue(ctx, fsctx.FileHeaderCtx, fileHeader)
			addFileCtx = context.WithValue(addFileCtx, fsctx.SavePathCtx, object.Source)

			// 尋找父目錄
			parentFolder := &model.Folder{}
			if parent, ok := pathCache[virtualPath]; ok {
				parentFolder = parent
			} else {
				exist, folder := fs.IsPathExist(virtualPath)
				if exist {
					parentFolder = folder
				} else {
					folder, err := fs.CreateDirectory(context.Background(), virtualPath)
					if err != nil {
						util.Log().Warning("匯入任務無法建立使用者目錄[%s], %s",
							virtualPath, err)
						continue
					}
					parentFolder = folder
				}
			}

			// 插入文件記錄
			_, err := fs.AddFile(addFileCtx, parentFolder)
			if err != nil {
				util.Log().Warning("匯入任務無法創插入文件[%s], %s",
					object.RelativePath, err)
				if err == filesystem.ErrInsufficientCapacity {
					job.SetErrorMsg("容量不足", err)
					return
				}
			}

		}
	}
}

// NewImportTask 建立匯入任務
func NewImportTask(user, policy uint, src, dst string, recursive bool) (Job, error) {
	creator, err := model.GetActiveUserByID(user)
	if err != nil {
		return nil, err
	}

	newTask := &ImportTask{
		User: &creator,
		TaskProps: ImportProps{
			PolicyID:  policy,
			Recursive: recursive,
			Src:       src,
			Dst:       dst,
		},
	}

	record, err := Record(newTask)
	if err != nil {
		return nil, err
	}
	newTask.TaskModel = record

	return newTask, nil
}

// NewImportTaskFromModel 從資料庫記錄中復原匯入任務
func NewImportTaskFromModel(task *model.Task) (Job, error) {
	user, err := model.GetActiveUserByID(task.UserID)
	if err != nil {
		return nil, err
	}
	newTask := &ImportTask{
		User:      &user,
		TaskModel: task,
	}

	err = json.Unmarshal([]byte(task.Props), &newTask.TaskProps)
	if err != nil {
		return nil, err
	}

	return newTask, nil
}
