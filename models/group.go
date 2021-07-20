package model

import (
	"encoding/json"
	"github.com/jinzhu/gorm"
)

// Group 使用者群組模型
type Group struct {
	gorm.Model
	Name          string
	Policies      string
	MaxStorage    uint64
	ShareEnabled  bool
	WebDAVEnabled bool
	SpeedLimit    int
	Options       string `json:"-",gorm:"type:text"`

	// 資料庫忽略欄位
	PolicyList        []uint      `gorm:"-"`
	OptionsSerialized GroupOption `gorm:"-"`
}

// GroupOption 使用者群組其他配置
type GroupOption struct {
	ArchiveDownload bool                   `json:"archive_download,omitempty"` // 打包下載
	ArchiveTask     bool                   `json:"archive_task,omitempty"`     // 線上壓縮
	CompressSize    uint64                 `json:"compress_size,omitempty"`    // 可壓縮大小
	DecompressSize  uint64                 `json:"decompress_size,omitempty"`
	OneTimeDownload bool                   `json:"one_time_download,omitempty"`
	ShareDownload   bool                   `json:"share_download,omitempty"`
	Aria2           bool                   `json:"aria2,omitempty"`         // 離線下載
	Aria2Options    map[string]interface{} `json:"aria2_options,omitempty"` // 離線下載使用者群組配置
}

// GetGroupByID 用ID獲取使用者群組
func GetGroupByID(ID interface{}) (Group, error) {
	var group Group
	result := DB.First(&group, ID)
	return group, result.Error
}

// AfterFind 找到使用者群組後的鉤子，處理Policy列表
func (group *Group) AfterFind() (err error) {
	// 解析使用者群組策略列表
	if group.Policies != "" {
		err = json.Unmarshal([]byte(group.Policies), &group.PolicyList)
	}
	if err != nil {
		return err
	}

	// 解析使用者群組設定
	if group.Options != "" {
		err = json.Unmarshal([]byte(group.Options), &group.OptionsSerialized)
	}

	return err
}

// BeforeSave Save使用者前的鉤子
func (group *Group) BeforeSave() (err error) {
	err = group.SerializePolicyList()
	return err
}

//SerializePolicyList 將序列後的可選策略列表、配置寫入資料庫欄位
// TODO 完善測試
func (group *Group) SerializePolicyList() (err error) {
	policies, err := json.Marshal(&group.PolicyList)
	group.Policies = string(policies)
	if err != nil {
		return err
	}

	optionsValue, err := json.Marshal(&group.OptionsSerialized)
	group.Options = string(optionsValue)
	return err
}
