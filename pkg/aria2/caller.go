package aria2

import (
	"context"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	model "github.com/cloudreve/Cloudreve/v3/models"
	"github.com/cloudreve/Cloudreve/v3/pkg/aria2/rpc"
	"github.com/cloudreve/Cloudreve/v3/pkg/util"
)

// RPCService 透過RPC服務的Aria2任務管理器
type RPCService struct {
	options *clientOptions
	Caller  rpc.Client
}

type clientOptions struct {
	Options map[string]interface{} // 建立下載時額外添加的設定
}

// Init 初始化
func (client *RPCService) Init(server, secret string, timeout int, options map[string]interface{}) error {
	// 用戶端已存在，則關閉先前連接
	if client.Caller != nil {
		client.Caller.Close()
	}

	client.options = &clientOptions{
		Options: options,
	}
	caller, err := rpc.New(context.Background(), server, secret, time.Duration(timeout)*time.Second,
		EventNotifier)
	client.Caller = caller
	return err
}

// Status 查詢下載狀態
func (client *RPCService) Status(task *model.Download) (rpc.StatusInfo, error) {
	res, err := client.Caller.TellStatus(task.GID)
	if err != nil {
		// 失敗後重試
		util.Log().Debug("無法獲取離線下載狀態，%s，10秒鐘後重試", err)
		time.Sleep(time.Duration(10) * time.Second)
		res, err = client.Caller.TellStatus(task.GID)
	}

	return res, err
}

// Cancel 取消下載
func (client *RPCService) Cancel(task *model.Download) error {
	// 取消下載任務
	_, err := client.Caller.Remove(task.GID)
	if err != nil {
		util.Log().Warning("無法取消離線下載任務[%s], %s", task.GID, err)
	}

	//// 刪除暫存檔
	//util.Log().Debug("離線下載任務[%s]已取消，1 分鐘後刪除暫存檔", task.GID)
	//go func(task *model.Download) {
	//	select {
	//	case <-time.After(time.Duration(60) * time.Second):
	//		err := os.RemoveAll(task.Parent)
	//		if err != nil {
	//			util.Log().Warning("無法刪除離線下載暫存資料夾[%s], %s", task.Parent, err)
	//		}
	//	}
	//}(task)

	return err
}

// Select 選取要下載的文件
func (client *RPCService) Select(task *model.Download, files []int) error {
	var selected = make([]string, len(files))
	for i := 0; i < len(files); i++ {
		selected[i] = strconv.Itoa(files[i])
	}
	_, err := client.Caller.ChangeOption(task.GID, map[string]interface{}{"select-file": strings.Join(selected, ",")})
	return err
}

// CreateTask 建立新任務
func (client *RPCService) CreateTask(task *model.Download, groupOptions map[string]interface{}) error {
	// 生成儲存路徑
	path := filepath.Join(
		model.GetSettingByName("aria2_temp_path"),
		"aria2",
		strconv.FormatInt(time.Now().UnixNano(), 10),
	)

	// 建立下載任務
	options := map[string]interface{}{
		"dir": path,
	}
	for k, v := range client.options.Options {
		options[k] = v
	}
	for k, v := range groupOptions {
		options[k] = v
	}

	gid, err := client.Caller.AddURI(task.Source, options)
	if err != nil || gid == "" {
		return err
	}

	// 儲存到資料庫
	task.GID = gid
	_, err = task.Create()
	if err != nil {
		return err
	}

	// 建立任務監控
	NewMonitor(task)

	return nil
}
