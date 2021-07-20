package task

import (
	model "github.com/cloudreve/Cloudreve/v3/models"
	"github.com/cloudreve/Cloudreve/v3/pkg/util"
)

// TaskPoll 要使用的任務池
var TaskPoll *Pool

// Pool 帶有最大配額的任務池
type Pool struct {
	// 容量
	idleWorker chan int
}

// Add 增加可用Worker數量
func (pool *Pool) Add(num int) {
	for i := 0; i < num; i++ {
		pool.idleWorker <- 1
	}
}

// ObtainWorker 阻塞直到獲取新的Worker
func (pool *Pool) ObtainWorker() Worker {
	select {
	case <-pool.idleWorker:
		// 有空閒Worker名額時，返回新Worker
		return &GeneralWorker{}
	}
}

// FreeWorker 添加空閒Worker
func (pool *Pool) FreeWorker() {
	pool.Add(1)
}

// Submit 開始提交任務
func (pool *Pool) Submit(job Job) {
	go func() {
		util.Log().Debug("等待獲取Worker")
		worker := pool.ObtainWorker()
		util.Log().Debug("獲取到Worker")
		worker.Do(job)
		util.Log().Debug("釋放Worker")
		pool.FreeWorker()
	}()
}

// Init 初始化任務池
func Init() {
	maxWorker := model.GetIntSetting("max_worker_num", 10)
	TaskPoll = &Pool{
		idleWorker: make(chan int, maxWorker),
	}
	TaskPoll.Add(maxWorker)
	util.Log().Info("初始化任務佇列，WorkerNum = %d", maxWorker)

	Resume()
}
