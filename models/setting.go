package model

import (
	"net/url"
	"strconv"

	"github.com/cloudreve/Cloudreve/v3/pkg/cache"
	"github.com/jinzhu/gorm"
)

// Setting 系統設定模型
type Setting struct {
	gorm.Model
	Type  string `gorm:"not null"`
	Name  string `gorm:"unique;not null;index:setting_key"`
	Value string `gorm:"size:‎65535"`
}

// IsTrueVal 返回設置的值是否為真
func IsTrueVal(val string) bool {
	return val == "1" || val == "true"
}

// GetSettingByName 用 Name 獲取設定值
func GetSettingByName(name string) string {
	var setting Setting

	// 優先從快取中尋找
	cacheKey := "setting_" + name
	if optionValue, ok := cache.Get(cacheKey); ok {
		return optionValue.(string)
	}
	// 嘗試資料庫中尋找
	result := DB.Where("name = ?", name).First(&setting)
	if result.Error == nil {
		_ = cache.Set(cacheKey, setting.Value, -1)
		return setting.Value
	}
	return ""
}

// GetSettingByNames 用多個 Name 獲取設定值
func GetSettingByNames(names ...string) map[string]string {
	var queryRes []Setting
	res, miss := cache.GetSettings(names, "setting_")

	if len(miss) > 0 {
		DB.Where("name IN (?)", miss).Find(&queryRes)
		for _, setting := range queryRes {
			res[setting.Name] = setting.Value
		}
	}

	_ = cache.SetSettings(res, "setting_")
	return res
}

// GetSettingByType 獲取一個或多個分組的所有設定值
func GetSettingByType(types []string) map[string]string {
	var queryRes []Setting
	res := make(map[string]string)

	DB.Where("type IN (?)", types).Find(&queryRes)
	for _, setting := range queryRes {
		res[setting.Name] = setting.Value
	}

	return res
}

// GetSiteURL 獲取站點地址
func GetSiteURL() *url.URL {
	base, err := url.Parse(GetSettingByName("siteURL"))
	if err != nil {
		base, _ = url.Parse("https://cloudreve.org")
	}
	return base
}

// GetIntSetting 獲取整形設定值，如果轉換失敗則返回預設值defaultVal
func GetIntSetting(key string, defaultVal int) int {
	res, err := strconv.Atoi(GetSettingByName(key))
	if err != nil {
		return defaultVal
	}
	return res
}
