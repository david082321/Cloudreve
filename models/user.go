package model

import (
	"crypto/md5"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"strings"

	"github.com/cloudreve/Cloudreve/v3/pkg/util"
	"github.com/jinzhu/gorm"
	"github.com/pkg/errors"
)

const (
	// Active 帳戶正常狀態
	Active = iota
	// NotActivicated 未啟動
	NotActivicated
	// Baned 被封禁
	Baned
	// OveruseBaned 超額使用被封禁
	OveruseBaned
)

// User 使用者模型
type User struct {
	// 表欄位
	gorm.Model
	Email     string `gorm:"type:varchar(100);unique_index"`
	Nick      string `gorm:"size:50"`
	Password  string `json:"-"`
	Status    int
	GroupID   uint
	Storage   uint64
	TwoFactor string
	Avatar    string
	Options   string `json:"-",gorm:"type:text"`
	Authn     string `gorm:"type:text"`

	// 關聯模型
	Group  Group  `gorm:"save_associations:false:false"`
	Policy Policy `gorm:"PRELOAD:false,association_autoupdate:false"`

	// 資料庫忽略欄位
	OptionsSerialized UserOption `gorm:"-"`
}

// UserOption 使用者個性化配置欄位
type UserOption struct {
	ProfileOff     bool   `json:"profile_off,omitempty"`
	PreferredTheme string `json:"preferred_theme,omitempty"`
}

// Root 獲取使用者的根目錄
func (user *User) Root() (*Folder, error) {
	var folder Folder
	err := DB.Where("parent_id is NULL AND owner_id = ?", user.ID).First(&folder).Error
	return &folder, err
}

// DeductionStorage 減少使用者已用容量
func (user *User) DeductionStorage(size uint64) bool {
	if size == 0 {
		return true
	}
	if size <= user.Storage {
		user.Storage -= size
		DB.Model(user).Update("storage", gorm.Expr("storage - ?", size))
		return true
	}
	// 如果要減少的容量超出已用容量，則設為零
	user.Storage = 0
	DB.Model(user).Update("storage", 0)

	return false
}

// IncreaseStorage 檢查並增加使用者已用容量
func (user *User) IncreaseStorage(size uint64) bool {
	if size == 0 {
		return true
	}
	if size <= user.GetRemainingCapacity() {
		user.Storage += size
		DB.Model(user).Update("storage", gorm.Expr("storage + ?", size))
		return true
	}
	return false
}

// IncreaseStorageWithoutCheck 忽略可用容量，增加使用者已用容量
func (user *User) IncreaseStorageWithoutCheck(size uint64) {
	if size == 0 {
		return
	}
	user.Storage += size
	DB.Model(user).Update("storage", gorm.Expr("storage + ?", size))

}

// GetRemainingCapacity 獲取剩餘配額
func (user *User) GetRemainingCapacity() uint64 {
	total := user.Group.MaxStorage
	if total <= user.Storage {
		return 0
	}
	return total - user.Storage
}

// GetPolicyID 獲取使用者目前的儲存策略ID
func (user *User) GetPolicyID(prefer uint) uint {
	if len(user.Group.PolicyList) > 0 {
		return user.Group.PolicyList[0]
	}
	return 0
}

// GetUserByID 用ID獲取使用者
func GetUserByID(ID interface{}) (User, error) {
	var user User
	result := DB.Set("gorm:auto_preload", true).First(&user, ID)
	return user, result.Error
}

// GetActiveUserByID 用ID獲取可登入使用者
func GetActiveUserByID(ID interface{}) (User, error) {
	var user User
	result := DB.Set("gorm:auto_preload", true).Where("status = ?", Active).First(&user, ID)
	return user, result.Error
}

// GetActiveUserByOpenID 用OpenID獲取可登入使用者
func GetActiveUserByOpenID(openid string) (User, error) {
	var user User
	result := DB.Set("gorm:auto_preload", true).Where("status = ? and open_id = ?", Active, openid).Find(&user)
	return user, result.Error
}

// GetUserByEmail 用Email獲取使用者
func GetUserByEmail(email string) (User, error) {
	var user User
	result := DB.Set("gorm:auto_preload", true).Where("email = ?", email).First(&user)
	return user, result.Error
}

// GetActiveUserByEmail 用Email獲取可登入使用者
func GetActiveUserByEmail(email string) (User, error) {
	var user User
	result := DB.Set("gorm:auto_preload", true).Where("status = ? and email = ?", Active, email).First(&user)
	return user, result.Error
}

// NewUser 返回一個新的空 User
func NewUser() User {
	options := UserOption{}
	return User{
		OptionsSerialized: options,
	}
}

// BeforeSave Save使用者前的鉤子
func (user *User) BeforeSave() (err error) {
	err = user.SerializeOptions()
	return err
}

// AfterCreate 建立使用者後的鉤子
func (user *User) AfterCreate(tx *gorm.DB) (err error) {
	// 建立使用者的預設根目錄
	defaultFolder := &Folder{
		Name:    "/",
		OwnerID: user.ID,
	}
	tx.Create(defaultFolder)
	return err
}

// AfterFind 找到使用者後的鉤子
func (user *User) AfterFind() (err error) {
	// 解析使用者設定到OptionsSerialized
	if user.Options != "" {
		err = json.Unmarshal([]byte(user.Options), &user.OptionsSerialized)
	}

	// 預載入儲存策略
	user.Policy, _ = GetPolicyByID(user.GetPolicyID(0))
	return err
}

//SerializeOptions 將序列後的Option寫入到資料庫欄位
func (user *User) SerializeOptions() (err error) {
	optionsValue, err := json.Marshal(&user.OptionsSerialized)
	user.Options = string(optionsValue)
	return err
}

// CheckPassword 根據明文校驗密碼
func (user *User) CheckPassword(password string) (bool, error) {

	// 根據儲存密碼分割為 Salt 和 Digest
	passwordStore := strings.Split(user.Password, ":")
	if len(passwordStore) != 2 && len(passwordStore) != 3 {
		return false, errors.New("Unknown password type")
	}

	// 相容V2密碼，升級後儲存格式為: md5:$HASH:$SALT
	if len(passwordStore) == 3 {
		if passwordStore[0] != "md5" {
			return false, errors.New("Unknown password type")
		}
		hash := md5.New()
		_, err := hash.Write([]byte(passwordStore[2] + password))
		bs := hex.EncodeToString(hash.Sum(nil))
		if err != nil {
			return false, err
		}
		return bs == passwordStore[1], nil
	}

	//計算 Salt 和密碼組合的SHA1摘要
	hash := sha1.New()
	_, err := hash.Write([]byte(password + passwordStore[0]))
	bs := hex.EncodeToString(hash.Sum(nil))
	if err != nil {
		return false, err
	}

	return bs == passwordStore[1], nil
}

// SetPassword 根據給定明文設定 User 的 Password 欄位
func (user *User) SetPassword(password string) error {
	//生成16位 Salt
	salt := util.RandStringRunes(16)

	//計算 Salt 和密碼組合的SHA1摘要
	hash := sha1.New()
	_, err := hash.Write([]byte(password + salt))
	bs := hex.EncodeToString(hash.Sum(nil))

	if err != nil {
		return err
	}

	//儲存 Salt 值和摘要， ":"分割
	user.Password = salt + ":" + string(bs)
	return nil
}

// NewAnonymousUser 返回一個匿名使用者
func NewAnonymousUser() *User {
	user := User{}
	user.Policy.Type = "anonymous"
	user.Group, _ = GetGroupByID(3)
	return &user
}

// IsAnonymous 返回是否為未登入使用者
func (user *User) IsAnonymous() bool {
	return user.ID == 0
}

// SetStatus 設定使用者狀態
func (user *User) SetStatus(status int) {
	DB.Model(&user).Update("status", status)
}

// Update 更新使用者
func (user *User) Update(val map[string]interface{}) error {
	return DB.Model(user).Updates(val).Error
}

// UpdateOptions 更新使用者偏好設定
func (user *User) UpdateOptions() error {
	if err := user.SerializeOptions(); err != nil {
		return err
	}
	return user.Update(map[string]interface{}{"options": user.Options})
}
