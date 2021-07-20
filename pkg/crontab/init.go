package crontab

import (
	model "github.com/cloudreve/Cloudreve/v3/models"
	"github.com/cloudreve/Cloudreve/v3/pkg/util"
	"github.com/robfig/cron/v3"
)

// Cron 定時任務
var Cron *cron.Cron

// Reload 重新啟動定時任務
func Reload() {
	if Cron != nil {
		Cron.Stop()
	}
	Init()
}

// Init 初始化定時任務
func Init() {
	util.Log().Info("初始化定時任務...")
	// 讀取cron日程設定
	options := model.GetSettingByNames("cron_garbage_collect")
	Cron := cron.New()
	for k, v := range options {
		var handler func()
		switch k {
		case "cron_garbage_collect":
			handler = garbageCollect
		default:
			util.Log().Warning("未知定時任務類型 [%s]，跳過", k)
			continue
		}

		if _, err := Cron.AddFunc(v, handler); err != nil {
			util.Log().Warning("無法啟動定時任務 [%s] , %s", k, err)
		}

	}
	Cron.Start()
}
