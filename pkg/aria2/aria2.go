package aria2

import (
	"encoding/json"
	"net/url"
	"sync"

	model "github.com/cloudreve/Cloudreve/v3/models"
	"github.com/cloudreve/Cloudreve/v3/pkg/aria2/rpc"
	"github.com/cloudreve/Cloudreve/v3/pkg/serializer"
	"github.com/cloudreve/Cloudreve/v3/pkg/util"
)

// Instance 預設使用的Aria2處理實例
var Instance Aria2 = &DummyAria2{}

// Lock Instance的讀寫鎖
var Lock sync.RWMutex

// EventNotifier 任務狀態更新通知處理器
var EventNotifier = &Notifier{}

// Aria2 離線下載處理介面
type Aria2 interface {
	// CreateTask 建立新的任務
	CreateTask(task *model.Download, options map[string]interface{}) error
	// 返回狀態訊息
	Status(task *model.Download) (rpc.StatusInfo, error)
	// 取消任務
	Cancel(task *model.Download) error
	// 選擇要下載的文件
	Select(task *model.Download, files []int) error
}

const (
	// URLTask 從URL添加的任務
	URLTask = iota
	// TorrentTask 種子任務
	TorrentTask
)

const (
	// Ready 準備就緒
	Ready = iota
	// Downloading 下載中
	Downloading
	// Paused 暫停中
	Paused
	// Error 出錯
	Error
	// Complete 完成
	Complete
	// Canceled 取消/停止
	Canceled
	// Unknown 未知狀態
	Unknown
)

var (
	// ErrNotEnabled 功能未開啟錯誤
	ErrNotEnabled = serializer.NewError(serializer.CodeNoPermissionErr, "離線下載功能未開啟", nil)
	// ErrUserNotFound 未找到下載任務建立者
	ErrUserNotFound = serializer.NewError(serializer.CodeNotFound, "無法找到任務建立者", nil)
)

// DummyAria2 未開啟Aria2功能時使用的預設處理器
type DummyAria2 struct {
}

// CreateTask 建立新任務，此處直接返回未開啟錯誤
func (instance *DummyAria2) CreateTask(model *model.Download, options map[string]interface{}) error {
	return ErrNotEnabled
}

// Status 返回未開啟錯誤
func (instance *DummyAria2) Status(task *model.Download) (rpc.StatusInfo, error) {
	return rpc.StatusInfo{}, ErrNotEnabled
}

// Cancel 返回未開啟錯誤
func (instance *DummyAria2) Cancel(task *model.Download) error {
	return ErrNotEnabled
}

// Select 返回未開啟錯誤
func (instance *DummyAria2) Select(task *model.Download, files []int) error {
	return ErrNotEnabled
}

// Init 初始化
func Init(isReload bool) {
	Lock.Lock()
	defer Lock.Unlock()

	// 關閉上個初始連接
	if previousClient, ok := Instance.(*RPCService); ok {
		if previousClient.Caller != nil {
			util.Log().Debug("關閉上個 aria2 連接")
			previousClient.Caller.Close()
		}
	}

	options := model.GetSettingByNames("aria2_rpcurl", "aria2_token", "aria2_options")
	timeout := model.GetIntSetting("aria2_call_timeout", 5)
	if options["aria2_rpcurl"] == "" {
		Instance = &DummyAria2{}
		return
	}

	util.Log().Info("初始化 aria2 RPC 服務[%s]", options["aria2_rpcurl"])
	client := &RPCService{}

	// 解析RPC服務地址
	server, err := url.Parse(options["aria2_rpcurl"])
	if err != nil {
		util.Log().Warning("無法解析 aria2 RPC 服務地址，%s", err)
		Instance = &DummyAria2{}
		return
	}
	server.Path = "/jsonrpc"

	// 載入自訂下載配置
	var globalOptions map[string]interface{}
	err = json.Unmarshal([]byte(options["aria2_options"]), &globalOptions)
	if err != nil {
		util.Log().Warning("無法解析 aria2 全域配置，%s", err)
		Instance = &DummyAria2{}
		return
	}

	if err := client.Init(server.String(), options["aria2_token"], timeout, globalOptions); err != nil {
		util.Log().Warning("初始化 aria2 RPC 服務失敗，%s", err)
		Instance = &DummyAria2{}
		return
	}

	Instance = client

	if !isReload {
		// 從資料庫中讀取未完成任務，建立監控
		unfinished := model.GetDownloadsByStatus(Ready, Paused, Downloading)

		for i := 0; i < len(unfinished); i++ {
			// 建立任務監控
			NewMonitor(&unfinished[i])
		}
	}

}

// getStatus 將給定的狀態字串轉換為狀態標識數字
func getStatus(status string) int {
	switch status {
	case "complete":
		return Complete
	case "active":
		return Downloading
	case "waiting":
		return Ready
	case "paused":
		return Paused
	case "error":
		return Error
	case "removed":
		return Canceled
	default:
		return Unknown
	}
}
