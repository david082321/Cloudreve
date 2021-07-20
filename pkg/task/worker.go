package task

import "github.com/cloudreve/Cloudreve/v3/pkg/util"

// Worker 處理任務的物件
type Worker interface {
	Do(Job) // 執行任務
}

// GeneralWorker 通用Worker
type GeneralWorker struct {
}

// Do 執行任務
func (worker *GeneralWorker) Do(job Job) {
	util.Log().Debug("開始執行任務")
	job.SetStatus(Processing)

	defer func() {
		// 致命錯誤捕獲
		if err := recover(); err != nil {
			util.Log().Debug("任務執行出錯，%s", err)
			job.SetError(&JobError{Msg: "致命錯誤"})
			job.SetStatus(Error)
		}
	}()

	// 開始執行任務
	job.Do()

	// 任務執行失敗
	if err := job.GetError(); err != nil {
		util.Log().Debug("任務執行出錯")
		job.SetStatus(Error)
		return
	}

	util.Log().Debug("任務執行完成")
	// 執行完成
	job.SetStatus(Complete)
}
