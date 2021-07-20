package main

import (
	"flag"

	"github.com/cloudreve/Cloudreve/v3/bootstrap"
	"github.com/cloudreve/Cloudreve/v3/pkg/conf"
	"github.com/cloudreve/Cloudreve/v3/pkg/util"
	"github.com/cloudreve/Cloudreve/v3/routers"
)

var (
	isEject    bool
	confPath   string
	scriptName string
)

func init() {
	flag.StringVar(&confPath, "c", util.RelativePath("conf.ini"), "配置檔案路徑")
	flag.BoolVar(&isEject, "eject", false, "匯出內建靜態資源")
	flag.StringVar(&scriptName, "database-script", "", "執行內建資料庫助手脚本")
	flag.Parse()
	bootstrap.Init(confPath)
}

func main() {
	if isEject {
		// 開始匯出內建靜態資來源文件
		bootstrap.Eject()
		return
	}

	if scriptName != "" {
		// 開始執行助手資料庫脚本
		bootstrap.RunScript(scriptName)
		return
	}

	api := routers.InitRouter()

	// 如果啟用了SSL
	if conf.SSLConfig.CertPath != "" {
		go func() {
			util.Log().Info("開始監聽 %s", conf.SSLConfig.Listen)
			if err := api.RunTLS(conf.SSLConfig.Listen,
				conf.SSLConfig.CertPath, conf.SSLConfig.KeyPath); err != nil {
				util.Log().Error("無法監聽[%s]，%s", conf.SSLConfig.Listen, err)
			}
		}()
	}

	// 如果啟用了Unix
	if conf.UnixConfig.Listen != "" {
		util.Log().Info("開始監聽 %s", conf.UnixConfig.Listen)
		if err := api.RunUnix(conf.UnixConfig.Listen); err != nil {
			util.Log().Error("無法監聽[%s]，%s", conf.UnixConfig.Listen, err)
		}
		return
	}

	util.Log().Info("開始監聽 %s", conf.SystemConfig.Listen)
	if err := api.Run(conf.SystemConfig.Listen); err != nil {
		util.Log().Error("無法監聽[%s]，%s", conf.SystemConfig.Listen, err)
	}
}
