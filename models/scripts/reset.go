package scripts

import (
	"context"
	model "github.com/cloudreve/Cloudreve/v3/models"
	"github.com/cloudreve/Cloudreve/v3/pkg/util"
	"github.com/fatih/color"
)

type ResetAdminPassword int

func init() {
	register("ResetAdminPassword", ResetAdminPassword(0))
}

// Run 執行脚本從社群版升級至 Pro 版
func (script ResetAdminPassword) Run(ctx context.Context) {
	// 尋找使用者
	user, err := model.GetUserByID(1)
	if err != nil {
		util.Log().Panic("初始管理員使用者不存在, %s", err)
	}

	// 生成密碼
	password := util.RandStringRunes(8)

	// 更改為新密碼
	user.SetPassword(password)
	if err := user.Update(map[string]interface{}{"password": user.Password}); err != nil {
		util.Log().Panic("密碼更改失敗, %s", err)
	}

	c := color.New(color.FgWhite).Add(color.BgBlack).Add(color.Bold)
	util.Log().Info("初始管理員密碼已更改為：" + c.Sprint(password))
}
