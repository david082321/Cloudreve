package email

import (
	"time"

	"github.com/cloudreve/Cloudreve/v3/pkg/util"
	"github.com/go-mail/mail"
)

// SMTP SMTP協議發送郵件
type SMTP struct {
	Config SMTPConfig
	ch     chan *mail.Message
	chOpen bool
}

// SMTPConfig SMTP發送配置
type SMTPConfig struct {
	Name       string // 發送者名
	Address    string // 發送者地址
	ReplyTo    string // 回復地址
	Host       string // 伺服器主機名稱
	Port       int    // 伺服器埠
	User       string // 使用者名稱
	Password   string // 密碼
	Encryption bool   // 是否啟用加密
	Keepalive  int    // SMTP 連接保留時長
}

// NewSMTPClient 建立SMTP發送佇列
func NewSMTPClient(config SMTPConfig) *SMTP {
	client := &SMTP{
		Config: config,
		ch:     make(chan *mail.Message, 30),
		chOpen: false,
	}

	client.Init()

	return client
}

// Send 發送郵件
func (client *SMTP) Send(to, title, body string) error {
	if !client.chOpen {
		return ErrChanNotOpen
	}
	m := mail.NewMessage()
	m.SetAddressHeader("From", client.Config.Address, client.Config.Name)
	m.SetAddressHeader("Reply-To", client.Config.ReplyTo, client.Config.Name)
	m.SetHeader("To", to)
	m.SetHeader("Subject", title)
	m.SetBody("text/html", body)
	client.ch <- m
	return nil
}

// Close 關閉發送佇列
func (client *SMTP) Close() {
	if client.ch != nil {
		close(client.ch)
	}
}

// Init 初始化發送佇列
func (client *SMTP) Init() {
	go func() {
		defer func() {
			if err := recover(); err != nil {
				client.chOpen = false
				util.Log().Error("郵件發送佇列出現異常, %s ,10 秒後重設", err)
				time.Sleep(time.Duration(10) * time.Second)
				client.Init()
			}
		}()

		d := mail.NewDialer(client.Config.Host, client.Config.Port, client.Config.User, client.Config.Password)
		d.Timeout = time.Duration(client.Config.Keepalive+5) * time.Second
		client.chOpen = true
		// 是否啟用 SSL
		d.SSL = false
		if client.Config.Encryption {
			d.SSL = true
		}
		d.StartTLSPolicy = mail.OpportunisticStartTLS

		var s mail.SendCloser
		var err error
		open := false
		for {
			select {
			case m, ok := <-client.ch:
				if !ok {
					util.Log().Debug("郵件佇列關閉")
					client.chOpen = false
					return
				}
				if !open {
					if s, err = d.Dial(); err != nil {
						panic(err)
					}
					open = true
				}
				if err := mail.Send(s, m); err != nil {
					util.Log().Warning("郵件發送失敗, %s", err)
				} else {
					util.Log().Debug("郵件已發送")
				}
			// 長時間沒有新郵件，則關閉SMTP連接
			case <-time.After(time.Duration(client.Config.Keepalive) * time.Second):
				if open {
					if err := s.Close(); err != nil {
						util.Log().Warning("無法關閉 SMTP 連接 %s", err)
					}
					open = false
				}
			}
		}
	}()
}
