package bootstrap

import (
	model "github.com/cloudreve/Cloudreve/v3/models"
	"github.com/cloudreve/Cloudreve/v3/pkg/aria2"
	"github.com/cloudreve/Cloudreve/v3/pkg/auth"
	"github.com/cloudreve/Cloudreve/v3/pkg/cache"
	"github.com/cloudreve/Cloudreve/v3/pkg/conf"
	"github.com/cloudreve/Cloudreve/v3/pkg/crontab"
	"github.com/cloudreve/Cloudreve/v3/pkg/email"
	"github.com/cloudreve/Cloudreve/v3/pkg/task"
	"github.com/gin-gonic/gin"
)

// Init 初始化啟動
func Init(path string) {
	InitApplication()
	conf.Init(path)
	// Debug 關閉時，切換為生產模式
	if !conf.SystemConfig.Debug {
		gin.SetMode(gin.ReleaseMode)
	}
	cache.Init()
	if conf.SystemConfig.Mode == "master" {
		model.Init()
		task.Init()
		aria2.Init(false)
		email.Init()
		crontab.Init()
		InitStatic()
	}
	auth.Init()
}
