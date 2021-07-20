package crontab

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	model "github.com/cloudreve/Cloudreve/v3/models"
	"github.com/cloudreve/Cloudreve/v3/pkg/cache"
	"github.com/cloudreve/Cloudreve/v3/pkg/util"
)

func garbageCollect() {
	// 清理打包下載產生的暫存檔
	collectArchiveFile()

	// 清理過期的內建記憶體快取
	if store, ok := cache.Store.(*cache.MemoStore); ok {
		collectCache(store)
	}

	util.Log().Info("定時任務 [cron_garbage_collect] 執行完畢")
}

func collectArchiveFile() {
	// 讀取有效期、目錄設定
	tempPath := util.RelativePath(model.GetSettingByName("temp_path"))
	expires := model.GetIntSetting("download_timeout", 30)

	// 列出文件
	root := filepath.Join(tempPath, "archive")
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() &&
			strings.HasPrefix(filepath.Base(path), "archive_") &&
			time.Now().Sub(info.ModTime()).Seconds() > float64(expires) {
			util.Log().Debug("刪除過期打包下載暫存檔 [%s]", path)
			// 刪除符合條件的文件
			if err := os.Remove(path); err != nil {
				util.Log().Debug("暫存檔 [%s] 刪除失敗 , %s", path, err)
			}
		}
		return nil
	})

	if err != nil {
		util.Log().Debug("[定時任務] 無法列取臨時打包目錄")
	}

}

func collectCache(store *cache.MemoStore) {
	util.Log().Debug("清理記憶體快取")
	store.GarbageCollect()
}
