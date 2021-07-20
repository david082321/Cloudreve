package aria2

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"time"

	model "github.com/cloudreve/Cloudreve/v3/models"
	"github.com/cloudreve/Cloudreve/v3/pkg/aria2/rpc"
	"github.com/cloudreve/Cloudreve/v3/pkg/filesystem"
	"github.com/cloudreve/Cloudreve/v3/pkg/filesystem/driver/local"
	"github.com/cloudreve/Cloudreve/v3/pkg/filesystem/fsctx"
	"github.com/cloudreve/Cloudreve/v3/pkg/task"
	"github.com/cloudreve/Cloudreve/v3/pkg/util"
)

// Monitor 離線下載狀態監控
type Monitor struct {
	Task     *model.Download
	Interval time.Duration

	notifier chan StatusEvent
	retried  int
}

// StatusEvent 狀態改變事件
type StatusEvent struct {
	GID    string
	Status int
}

var MAX_RETRY = 10

// NewMonitor 建立上傳狀態監控
func NewMonitor(task *model.Download) {
	monitor := &Monitor{
		Task:     task,
		Interval: time.Duration(model.GetIntSetting("aria2_interval", 10)) * time.Second,
		notifier: make(chan StatusEvent),
	}
	go monitor.Loop()
	EventNotifier.Subscribe(monitor.notifier, monitor.Task.GID)
}

// Loop 開啟監控循環
func (monitor *Monitor) Loop() {
	defer EventNotifier.Unsubscribe(monitor.Task.GID)

	// 首次循環立即更新
	interval := time.Duration(0)

	for {
		select {
		case <-monitor.notifier:
			if monitor.Update() {
				return
			}
		case <-time.After(interval):
			interval = monitor.Interval
			if monitor.Update() {
				return
			}
		}
	}
}

// Update 更新狀態，返回值表示是否退出監控
func (monitor *Monitor) Update() bool {
	Lock.RLock()
	status, err := Instance.Status(monitor.Task)
	Lock.RUnlock()

	if err != nil {
		monitor.retried++
		util.Log().Warning("無法獲取下載任務[%s]的狀態，%s", monitor.Task.GID, err)

		// 十次重試後認定為任務失敗
		if monitor.retried > MAX_RETRY {
			util.Log().Warning("無法獲取下載任務[%s]的狀態，超過最大重試次數限制，%s", monitor.Task.GID, err)
			monitor.setErrorStatus(err)
			monitor.RemoveTempFolder()
			return true
		}

		return false
	}
	monitor.retried = 0

	// 磁力鏈下載需要跟隨
	if len(status.FollowedBy) > 0 {
		util.Log().Debug("離線下載[%s]重定向至[%s]", monitor.Task.GID, status.FollowedBy[0])
		monitor.Task.GID = status.FollowedBy[0]
		monitor.Task.Save()
		return false
	}

	// 更新任務訊息
	if err := monitor.UpdateTaskInfo(status); err != nil {
		util.Log().Warning("無法更新下載任務[%s]的任務訊息[%s]，", monitor.Task.GID, err)
		monitor.setErrorStatus(err)
		return true
	}

	util.Log().Debug("離線下載[%s]更新狀態[%s]", status.Gid, status.Status)

	switch status.Status {
	case "complete":
		return monitor.Complete(status)
	case "error":
		return monitor.Error(status)
	case "active", "waiting", "paused":
		return false
	case "removed":
		monitor.Task.Status = Canceled
		monitor.Task.Save()
		monitor.RemoveTempFolder()
		return true
	default:
		util.Log().Warning("下載任務[%s]返回未知狀態訊息[%s]，", monitor.Task.GID, status.Status)
		return true
	}
}

// UpdateTaskInfo 更新資料庫中的任務訊息
func (monitor *Monitor) UpdateTaskInfo(status rpc.StatusInfo) error {
	originSize := monitor.Task.TotalSize

	monitor.Task.GID = status.Gid
	monitor.Task.Status = getStatus(status.Status)

	// 檔案大小、已下載大小
	total, err := strconv.ParseUint(status.TotalLength, 10, 64)
	if err != nil {
		total = 0
	}
	downloaded, err := strconv.ParseUint(status.CompletedLength, 10, 64)
	if err != nil {
		downloaded = 0
	}
	monitor.Task.TotalSize = total
	monitor.Task.DownloadedSize = downloaded
	monitor.Task.GID = status.Gid
	monitor.Task.Parent = status.Dir

	// 下載速度
	speed, err := strconv.Atoi(status.DownloadSpeed)
	if err != nil {
		speed = 0
	}

	monitor.Task.Speed = speed
	attrs, _ := json.Marshal(status)
	monitor.Task.Attrs = string(attrs)

	if err := monitor.Task.Save(); err != nil {
		return err
	}

	if originSize != monitor.Task.TotalSize {
		// 檔案大小更新後，對文件限制等進行校驗
		if err := monitor.ValidateFile(); err != nil {
			// 驗證失敗時取消任務
			Lock.RLock()
			Instance.Cancel(monitor.Task)
			Lock.RUnlock()
			return err
		}
	}

	return nil
}

// ValidateFile 上傳過程中校驗檔案大小、檔案名
func (monitor *Monitor) ValidateFile() error {
	// 找到任務建立者
	user := monitor.Task.GetOwner()
	if user == nil {
		return ErrUserNotFound
	}

	// 建立文件系統
	fs, err := filesystem.NewFileSystem(user)
	if err != nil {
		return err
	}
	defer fs.Recycle()

	// 建立上下文環境
	ctx := context.WithValue(context.Background(), fsctx.FileHeaderCtx, local.FileStream{
		Size: monitor.Task.TotalSize,
	})

	// 驗證使用者容量
	if err := filesystem.HookValidateCapacityWithoutIncrease(ctx, fs); err != nil {
		return err
	}

	// 驗證每個文件
	for _, fileInfo := range monitor.Task.StatusInfo.Files {
		if fileInfo.Selected == "true" {
			// 建立上下文環境
			fileSize, _ := strconv.ParseUint(fileInfo.Length, 10, 64)
			ctx := context.WithValue(context.Background(), fsctx.FileHeaderCtx, local.FileStream{
				Size: fileSize,
				Name: filepath.Base(fileInfo.Path),
			})
			if err := filesystem.HookValidateFile(ctx, fs); err != nil {
				return err
			}
		}

	}

	return nil
}

// Error 任務下載出錯處理，返回是否中斷監控
func (monitor *Monitor) Error(status rpc.StatusInfo) bool {
	monitor.setErrorStatus(errors.New(status.ErrorMessage))

	// 清理暫存檔
	monitor.RemoveTempFolder()

	return true
}

// RemoveTempFolder 清理下載暫存資料夾
func (monitor *Monitor) RemoveTempFolder() {
	err := os.RemoveAll(monitor.Task.Parent)
	if err != nil {
		util.Log().Warning("無法刪除離線下載暫存資料夾[%s], %s", monitor.Task.Parent, err)
	}

}

// Complete 完成下載，返回是否中斷監控
func (monitor *Monitor) Complete(status rpc.StatusInfo) bool {
	// 建立中轉任務
	file := make([]string, 0, len(monitor.Task.StatusInfo.Files))
	for i := 0; i < len(monitor.Task.StatusInfo.Files); i++ {
		if monitor.Task.StatusInfo.Files[i].Selected == "true" {
			file = append(file, monitor.Task.StatusInfo.Files[i].Path)
		}
	}
	job, err := task.NewTransferTask(
		monitor.Task.UserID,
		file,
		monitor.Task.Dst,
		monitor.Task.Parent,
		true,
	)
	if err != nil {
		monitor.setErrorStatus(err)
		return true
	}

	// 提交中轉任務
	task.TaskPoll.Submit(job)

	// 更新任務ID
	monitor.Task.TaskID = job.Model().ID
	monitor.Task.Save()

	return true
}

func (monitor *Monitor) setErrorStatus(err error) {
	monitor.Task.Status = Error
	monitor.Task.Error = err.Error()
	monitor.Task.Save()
}
