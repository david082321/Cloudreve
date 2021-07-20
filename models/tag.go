package model

import (
	"github.com/cloudreve/Cloudreve/v3/pkg/util"
	"github.com/jinzhu/gorm"
)

// Tag 使用者自訂標籤
type Tag struct {
	gorm.Model
	Name       string // 標籤名
	Icon       string // 圖示標識
	Color      string // 圖示顏色
	Type       int    // 標籤類型（文件分類/目錄直達）
	Expression string `gorm:"type:text"` // 搜尋表表達式/直達路徑
	UserID     uint   // 建立者ID
}

const (
	// FileTagType 文件分類標籤
	FileTagType = iota
	// DirectoryLinkType 目錄捷徑標籤
	DirectoryLinkType
)

// Create 建立標籤記錄
func (tag *Tag) Create() (uint, error) {
	if err := DB.Create(tag).Error; err != nil {
		util.Log().Warning("無法插入離線下載記錄, %s", err)
		return 0, err
	}
	return tag.ID, nil
}

// DeleteTagByID 根據給定ID和使用者ID刪除標籤
func DeleteTagByID(id, uid uint) error {
	result := DB.Where("id = ? and user_id = ?", id, uid).Delete(&Tag{})
	return result.Error
}

// GetTagsByUID 根據使用者ID尋找標籤
func GetTagsByUID(uid uint) ([]Tag, error) {
	var tag []Tag
	result := DB.Where("user_id = ?", uid).Find(&tag)
	return tag, result.Error
}

// GetTagsByID 根據ID尋找標籤
func GetTagsByID(id, uid uint) (*Tag, error) {
	var tag Tag
	result := DB.Where("user_id = ? and id = ?", uid, id).First(&tag)
	return &tag, result.Error
}
