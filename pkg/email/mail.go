package email

import (
	"errors"
	"strings"
)

// Driver 郵件發送驅動
type Driver interface {
	// Close 關閉驅動
	Close()
	// Send 發送郵件
	Send(to, title, body string) error
}

var (
	// ErrChanNotOpen 郵件佇列未開啟
	ErrChanNotOpen = errors.New("郵件佇列未開啟")
	// ErrNoActiveDriver 無可用郵件發送服務
	ErrNoActiveDriver = errors.New("無可用郵件發送服務")
)

// Send 發送郵件
func Send(to, title, body string) error {
	// 忽略透過QQ登入的信箱
	if strings.HasSuffix(to, "@login.qq.com") {
		return nil
	}

	Lock.RLock()
	defer Lock.RUnlock()

	if Client == nil {
		return ErrNoActiveDriver
	}

	return Client.Send(to, title, body)
}
