package aria2

import (
	"sync"

	"github.com/cloudreve/Cloudreve/v3/pkg/aria2/rpc"
)

// Notifier aria2實踐通知處理
type Notifier struct {
	Subscribes sync.Map
}

// Subscribe 訂閱事件通知
func (notifier *Notifier) Subscribe(target chan StatusEvent, gid string) {
	notifier.Subscribes.Store(gid, target)
}

// Unsubscribe 取消訂閱事件通知
func (notifier *Notifier) Unsubscribe(gid string) {
	notifier.Subscribes.Delete(gid)
}

// Notify 發送通知
func (notifier *Notifier) Notify(events []rpc.Event, status int) {
	for _, event := range events {
		if target, ok := notifier.Subscribes.Load(event.Gid); ok {
			target.(chan StatusEvent) <- StatusEvent{
				GID:    event.Gid,
				Status: status,
			}
		}
	}
}

// OnDownloadStart 下載開始
func (notifier *Notifier) OnDownloadStart(events []rpc.Event) {
	notifier.Notify(events, Downloading)
}

// OnDownloadPause 下載暫停
func (notifier *Notifier) OnDownloadPause(events []rpc.Event) {
	notifier.Notify(events, Paused)
}

// OnDownloadStop 下載停止
func (notifier *Notifier) OnDownloadStop(events []rpc.Event) {
	notifier.Notify(events, Canceled)
}

// OnDownloadComplete 下載完成
func (notifier *Notifier) OnDownloadComplete(events []rpc.Event) {
	notifier.Notify(events, Complete)
}

// OnDownloadError 下載出錯
func (notifier *Notifier) OnDownloadError(events []rpc.Event) {
	notifier.Notify(events, Error)
}

// OnBtDownloadComplete BT下載完成
func (notifier *Notifier) OnBtDownloadComplete(events []rpc.Event) {
	notifier.Notify(events, Complete)
}
