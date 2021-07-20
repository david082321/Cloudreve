package scripts

import (
	"context"
	model "github.com/cloudreve/Cloudreve/v3/models"
	"github.com/cloudreve/Cloudreve/v3/pkg/util"
)

type UserStorageCalibration int

func init() {
	register("CalibrateUserStorage", UserStorageCalibration(0))
}

type storageResult struct {
	Total uint64
}

// Run 執行脚本校準所有使用者容量
func (script UserStorageCalibration) Run(ctx context.Context) {
	// 列出所有使用者
	var res []model.User
	model.DB.Model(&model.User{}).Find(&res)

	// 逐個檢查容量
	for _, user := range res {
		// 計算正確的容量
		var total storageResult
		model.DB.Model(&model.File{}).Where("user_id = ?", user.ID).Select("sum(size) as total").Scan(&total)
		// 更新使用者的容量
		if user.Storage != total.Total {
			util.Log().Info("將使用者 [%s] 的容量由 %d 校準為 %d", user.Email,
				user.Storage, total.Total)
			model.DB.Model(&user).Update("storage", total.Total)
		}
	}
}
