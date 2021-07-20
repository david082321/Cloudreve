package model

import (
	"github.com/jinzhu/gorm"
)

// Webdav 應用帳戶
type Webdav struct {
	gorm.Model
	Name     string // 應用名稱
	Password string `gorm:"unique_index:password_only_on"` // 應用密碼
	UserID   uint   `gorm:"unique_index:password_only_on"` // 使用者ID
	Root     string `gorm:"type:text"`                     // 根目錄
}

// Create 建立帳戶
func (webdav *Webdav) Create() (uint, error) {
	if err := DB.Create(webdav).Error; err != nil {
		return 0, err
	}
	return webdav.ID, nil
}

// GetWebdavByPassword 根據密碼和使用者尋找Webdav應用
func GetWebdavByPassword(password string, uid uint) (*Webdav, error) {
	webdav := &Webdav{}
	res := DB.Where("user_id = ? and password = ?", uid, password).First(webdav)
	return webdav, res.Error
}

// ListWebDAVAccounts 列出使用者的所有帳號
func ListWebDAVAccounts(uid uint) []Webdav {
	var accounts []Webdav
	DB.Where("user_id = ?", uid).Order("created_at desc").Find(&accounts)
	return accounts
}

// DeleteWebDAVAccountByID 根據帳戶ID和UID刪除帳戶
func DeleteWebDAVAccountByID(id, uid uint) {
	DB.Where("user_id = ? and id = ?", uid, id).Delete(&Webdav{})
}
