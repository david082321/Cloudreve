package bootstrap

import (
	"context"
	"github.com/cloudreve/Cloudreve/v3/models/scripts"
	"github.com/cloudreve/Cloudreve/v3/pkg/util"
)

func RunScript(name string) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := scripts.RunDBScript(name, ctx); err != nil {
		util.Log().Error("資料庫脚本執行失敗: %s", err)
		return
	}

	util.Log().Info("資料庫脚本 [%s] 執行完畢", name)
}
