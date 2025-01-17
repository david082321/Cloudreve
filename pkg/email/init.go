package email

import (
	"sync"

	model "github.com/cloudreve/Cloudreve/v3/models"
	"github.com/cloudreve/Cloudreve/v3/pkg/util"
)

// Client 預設的郵件發送用戶端
var Client Driver

// Lock 讀寫鎖
var Lock sync.RWMutex

// Init 初始化
func Init() {
	util.Log().Debug("郵件佇列初始化")
	Lock.Lock()
	defer Lock.Unlock()

	if Client != nil {
		Client.Close()
	}

	// 讀取SMTP設定
	options := model.GetSettingByNames(
		"fromName",
		"fromAdress",
		"smtpHost",
		"replyTo",
		"smtpUser",
		"smtpPass",
		"smtpEncryption",
	)
	port := model.GetIntSetting("smtpPort", 25)
	keepAlive := model.GetIntSetting("mail_keepalive", 30)

	client := NewSMTPClient(SMTPConfig{
		Name:       options["fromName"],
		Address:    options["fromAdress"],
		ReplyTo:    options["replyTo"],
		Host:       options["smtpHost"],
		Port:       port,
		User:       options["smtpUser"],
		Password:   options["smtpPass"],
		Keepalive:  keepAlive,
		Encryption: model.IsTrueVal(options["smtpEncryption"]),
	})

	Client = client
}
