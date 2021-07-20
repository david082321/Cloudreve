package model

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cloudreve/Cloudreve/v3/pkg/cache"
	"github.com/cloudreve/Cloudreve/v3/pkg/hashid"
	"github.com/cloudreve/Cloudreve/v3/pkg/util"
	"github.com/gin-gonic/gin"
	"github.com/jinzhu/gorm"
)

// Share 分享模型
type Share struct {
	gorm.Model
	Password        string     // 分享密碼，空值為非加密分享
	IsDir           bool       // 原始資源是否為目錄
	UserID          uint       // 建立使用者ID
	SourceID        uint       // 原始資源ID
	Views           int        // 瀏覽數
	Downloads       int        // 下載數
	RemainDownloads int        // 剩餘下載配額，負值標識無限制
	Expires         *time.Time // 過期時間，空值表示無過期時間
	PreviewEnabled  bool       // 是否允許直接預覽
	SourceName      string     `gorm:"index:source"` // 用於搜尋的欄位

	// 資料庫忽略欄位
	User   User   `gorm:"PRELOAD:false,association_autoupdate:false"`
	File   File   `gorm:"PRELOAD:false,association_autoupdate:false"`
	Folder Folder `gorm:"PRELOAD:false,association_autoupdate:false"`
}

// Create 建立分享
func (share *Share) Create() (uint, error) {
	if err := DB.Create(share).Error; err != nil {
		util.Log().Warning("無法插入資料庫記錄, %s", err)
		return 0, err
	}
	return share.ID, nil
}

// GetShareByHashID 根據HashID尋找分享
func GetShareByHashID(hashID string) *Share {
	id, err := hashid.DecodeHashID(hashID, hashid.ShareID)
	if err != nil {
		return nil
	}
	var share Share
	result := DB.First(&share, id)
	if result.Error != nil {
		return nil
	}

	return &share
}

// IsAvailable 返回此分享是否可用（是否過期）
func (share *Share) IsAvailable() bool {
	if share.RemainDownloads == 0 {
		return false
	}
	if share.Expires != nil && time.Now().After(*share.Expires) {
		return false
	}

	// 檢查建立者狀態
	if share.Creator().Status != Active {
		return false
	}

	// 檢查源物件是否存在
	var sourceID uint
	if share.IsDir {
		folder := share.SourceFolder()
		sourceID = folder.ID
	} else {
		file := share.SourceFile()
		sourceID = file.ID
	}
	if sourceID == 0 {
		// TODO 是否要在這裡刪除這個無效分享？
		return false
	}

	return true
}

// Creator 獲取分享的建立者
func (share *Share) Creator() *User {
	if share.User.ID == 0 {
		share.User, _ = GetUserByID(share.UserID)
	}
	return &share.User
}

// Source 返回源物件
func (share *Share) Source() interface{} {
	if share.IsDir {
		return share.SourceFolder()
	}
	return share.SourceFile()
}

// SourceFolder 獲取源目錄
func (share *Share) SourceFolder() *Folder {
	if share.Folder.ID == 0 {
		folders, _ := GetFoldersByIDs([]uint{share.SourceID}, share.UserID)
		if len(folders) > 0 {
			share.Folder = folders[0]
		}
	}
	return &share.Folder
}

// SourceFile 獲取來源文件
func (share *Share) SourceFile() *File {
	if share.File.ID == 0 {
		files, _ := GetFilesByIDs([]uint{share.SourceID}, share.UserID)
		if len(files) > 0 {
			share.File = files[0]
		}
	}
	return &share.File
}

// CanBeDownloadBy 返回此分享是否可以被給定使用者下載
func (share *Share) CanBeDownloadBy(user *User) error {
	// 使用者群組權限
	if !user.Group.OptionsSerialized.ShareDownload {
		if user.IsAnonymous() {
			return errors.New("未登入使用者無法下載")
		}
		return errors.New("您目前的使用者群組無權下載")
	}
	return nil
}

// WasDownloadedBy 返回分享是否已被使用者下載過
func (share *Share) WasDownloadedBy(user *User, c *gin.Context) (exist bool) {
	if user.IsAnonymous() {
		exist = util.GetSession(c, fmt.Sprintf("share_%d_%d", share.ID, user.ID)) != nil
	} else {
		_, exist = cache.Get(fmt.Sprintf("share_%d_%d", share.ID, user.ID))
	}

	return exist
}

// DownloadBy 增加下載次數，匿名使用者不會快取
func (share *Share) DownloadBy(user *User, c *gin.Context) error {
	if !share.WasDownloadedBy(user, c) {
		share.Downloaded()
		if !user.IsAnonymous() {
			cache.Set(fmt.Sprintf("share_%d_%d", share.ID, user.ID), true,
				GetIntSetting("share_download_session_timeout", 2073600))
		} else {
			util.SetSession(c, map[string]interface{}{fmt.Sprintf("share_%d_%d", share.ID, user.ID): true})
		}
	}
	return nil
}

// Viewed 增加訪問次數
func (share *Share) Viewed() {
	share.Views++
	DB.Model(share).UpdateColumn("views", gorm.Expr("views + ?", 1))
}

// Downloaded 增加下載次數
func (share *Share) Downloaded() {
	share.Downloads++
	if share.RemainDownloads > 0 {
		share.RemainDownloads--
	}
	DB.Model(share).Updates(map[string]interface{}{
		"downloads":        share.Downloads,
		"remain_downloads": share.RemainDownloads,
	})
}

// Update 更新分享屬性
func (share *Share) Update(props map[string]interface{}) error {
	return DB.Model(share).Updates(props).Error
}

// Delete 刪除分享
func (share *Share) Delete() error {
	return DB.Model(share).Delete(share).Error
}

// DeleteShareBySourceIDs 根據原始資源類型和ID刪除文件
func DeleteShareBySourceIDs(sources []uint, isDir bool) error {
	return DB.Where("source_id in (?) and is_dir = ?", sources, isDir).Delete(&Share{}).Error
}

// ListShares 列出UID下的分享
func ListShares(uid uint, page, pageSize int, order string, publicOnly bool) ([]Share, int) {
	var (
		shares []Share
		total  int
	)
	dbChain := DB
	dbChain = dbChain.Where("user_id = ?", uid)
	if publicOnly {
		dbChain = dbChain.Where("password = ?", "")
	}

	// 計算總數用於分頁
	dbChain.Model(&Share{}).Count(&total)

	// 查詢記錄
	dbChain.Limit(pageSize).Offset((page - 1) * pageSize).Order(order).Find(&shares)
	return shares, total
}

// SearchShares 根據關鍵字搜尋分享
func SearchShares(page, pageSize int, order, keywords string) ([]Share, int) {
	var (
		shares []Share
		total  int
	)

	keywordList := strings.Split(keywords, " ")
	availableList := make([]string, 0, len(keywordList))
	for i := 0; i < len(keywordList); i++ {
		if len(keywordList[i]) > 0 {
			availableList = append(availableList, keywordList[i])
		}
	}
	if len(availableList) == 0 {
		return shares, 0
	}

	dbChain := DB
	dbChain = dbChain.Where("password = ? and remain_downloads <> 0 and (expires is NULL or expires > ?) and source_name like ?", "", time.Now(), "%"+strings.Join(availableList, "%")+"%")

	// 計算總數用於分頁
	dbChain.Model(&Share{}).Count(&total)

	// 查詢記錄
	dbChain.Limit(pageSize).Offset((page - 1) * pageSize).Order(order).Find(&shares)
	return shares, total
}
