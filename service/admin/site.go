package admin

import (
	"time"

	model "github.com/cloudreve/Cloudreve/v3/models"
	"github.com/cloudreve/Cloudreve/v3/pkg/cache"
	"github.com/cloudreve/Cloudreve/v3/pkg/conf"
	"github.com/cloudreve/Cloudreve/v3/pkg/email"
	"github.com/cloudreve/Cloudreve/v3/pkg/serializer"
)

// NoParamService 無需參數的服務
type NoParamService struct {
}

// BatchSettingChangeService 設定批次更改服務
type BatchSettingChangeService struct {
	Options []SettingChangeService `json:"options"`
}

// SettingChangeService  設定更改服務
type SettingChangeService struct {
	Key   string `json:"key" binding:"required"`
	Value string `json:"value"`
}

// BatchSettingGet 設定批次獲取服務
type BatchSettingGet struct {
	Keys []string `json:"keys"`
}

// MailTestService 郵件測試服務
type MailTestService struct {
	Email string `json:"to" binding:"email"`
}

// Send 發送測試郵件
func (service *MailTestService) Send() serializer.Response {
	if err := email.Send(service.Email, "Cloudreve發信測試", "這是一封測試郵件，用於測試 Cloudreve 發信設定。"); err != nil {
		return serializer.Err(serializer.CodeInternalSetting, "發信失敗, "+err.Error(), nil)
	}
	return serializer.Response{}
}

// Get 獲取設定值
func (service *BatchSettingGet) Get() serializer.Response {
	options := model.GetSettingByNames(service.Keys...)
	return serializer.Response{Data: options}
}

// Change 批次更改站點設定
func (service *BatchSettingChangeService) Change() serializer.Response {
	cacheClean := make([]string, 0, len(service.Options))

	for _, setting := range service.Options {

		if err := model.DB.Model(&model.Setting{}).Where("name = ?", setting.Key).Update("value", setting.Value).Error; err != nil {
			cache.Deletes(cacheClean, "setting_")
			return serializer.DBErr("設定 "+setting.Key+" 更新失敗", err)
		}

		cacheClean = append(cacheClean, setting.Key)
	}

	cache.Deletes(cacheClean, "setting_")

	return serializer.Response{}
}

// Summary 獲取站點統計概況
func (service *NoParamService) Summary() serializer.Response {
	// 統計每日概況
	total := 12
	files := make([]int, total)
	users := make([]int, total)
	shares := make([]int, total)
	date := make([]string, total)

	toRound := time.Now()
	timeBase := time.Date(toRound.Year(), toRound.Month(), toRound.Day()+1, 0, 0, 0, 0, toRound.Location())
	for day := range files {
		start := timeBase.Add(-time.Duration(total-day) * time.Hour * 24)
		end := timeBase.Add(-time.Duration(total-day-1) * time.Hour * 24)
		date[day] = start.Format("1月2日")
		model.DB.Model(&model.User{}).Where("created_at BETWEEN ? AND ?", start, end).Count(&users[day])
		model.DB.Model(&model.File{}).Where("created_at BETWEEN ? AND ?", start, end).Count(&files[day])
		model.DB.Model(&model.Share{}).Where("created_at BETWEEN ? AND ?", start, end).Count(&shares[day])
	}

	// 統計總數
	fileTotal := 0
	userTotal := 0
	publicShareTotal := 0
	secretShareTotal := 0
	model.DB.Model(&model.User{}).Count(&userTotal)
	model.DB.Model(&model.File{}).Count(&fileTotal)
	model.DB.Model(&model.Share{}).Where("password = ?", "").Count(&publicShareTotal)
	model.DB.Model(&model.Share{}).Where("password <> ?", "").Count(&secretShareTotal)

	// 獲取版本訊息
	versions := map[string]string{
		"backend": conf.BackendVersion,
		"db":      conf.RequiredDBVersion,
		"commit":  conf.LastCommit,
		"is_pro":  conf.IsPro,
	}

	return serializer.Response{
		Data: map[string]interface{}{
			"date":             date,
			"files":            files,
			"users":            users,
			"shares":           shares,
			"version":          versions,
			"siteURL":          model.GetSettingByName("siteURL"),
			"fileTotal":        fileTotal,
			"userTotal":        userTotal,
			"publicShareTotal": publicShareTotal,
			"secretShareTotal": secretShareTotal,
		},
	}
}
