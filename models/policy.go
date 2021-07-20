package model

import (
	"encoding/gob"
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/cloudreve/Cloudreve/v3/pkg/cache"
	"github.com/cloudreve/Cloudreve/v3/pkg/util"
	"github.com/jinzhu/gorm"
)

// Policy 儲存策略
type Policy struct {
	// 表欄位
	gorm.Model
	Name               string
	Type               string
	Server             string
	BucketName         string
	IsPrivate          bool
	BaseURL            string
	AccessKey          string `gorm:"type:text"`
	SecretKey          string `gorm:"type:text"`
	MaxSize            uint64
	AutoRename         bool
	DirNameRule        string
	FileNameRule       string
	IsOriginLinkEnable bool
	Options            string `gorm:"type:text"`

	// 資料庫忽略欄位
	OptionsSerialized PolicyOption `gorm:"-"`
}

// PolicyOption 非公有的儲存策略屬性
type PolicyOption struct {
	// Upyun訪問Token
	Token string `json:"token"`
	// 允許的文件副檔名
	FileType []string `json:"file_type"`
	// MimeType
	MimeType string `json:"mimetype"`
	// OdRedirect Onedrive 重定向地址
	OdRedirect string `json:"od_redirect,omitempty"`
	// OdProxy Onedrive 反代地址
	OdProxy string `json:"od_proxy,omitempty"`
	// OdDriver OneDrive 驅動器定位符
	OdDriver string `json:"od_driver,omitempty"`
	// Region 區域代碼
	Region string `json:"region,omitempty"`
	// ServerSideEndpoint 服務端請求使用的 Endpoint，為空時使用 Policy.Server 欄位
	ServerSideEndpoint string `json:"server_side_endpoint,omitempty"`
}

var thumbSuffix = map[string][]string{
	"local":    {},
	"qiniu":    {".psd", ".jpg", ".jpeg", ".png", ".gif", ".webp", ".tiff", ".bmp"},
	"oss":      {".jpg", ".jpeg", ".png", ".gif", ".webp", ".tiff", ".bmp"},
	"cos":      {".jpg", ".jpeg", ".png", ".gif", ".webp", ".tiff", ".bmp"},
	"upyun":    {".svg", ".jpg", ".jpeg", ".png", ".gif", ".webp", ".tiff", ".bmp"},
	"s3":       {},
	"remote":   {},
	"onedrive": {"*"},
}

func init() {
	// 註冊快取用到的複雜結構
	gob.Register(Policy{})
}

// GetPolicyByID 用ID獲取儲存策略
func GetPolicyByID(ID interface{}) (Policy, error) {
	// 嘗試讀取快取
	cacheKey := "policy_" + strconv.Itoa(int(ID.(uint)))
	if policy, ok := cache.Get(cacheKey); ok {
		return policy.(Policy), nil
	}

	var policy Policy
	result := DB.First(&policy, ID)

	// 寫入快取
	if result.Error == nil {
		_ = cache.Set(cacheKey, policy, -1)
	}

	return policy, result.Error
}

// AfterFind 找到儲存策略後的鉤子
func (policy *Policy) AfterFind() (err error) {
	// 解析儲存策略設定到OptionsSerialized
	if policy.Options != "" {
		err = json.Unmarshal([]byte(policy.Options), &policy.OptionsSerialized)
	}
	if policy.OptionsSerialized.FileType == nil {
		policy.OptionsSerialized.FileType = []string{}
	}

	return err
}

// BeforeSave Save策略前的鉤子
func (policy *Policy) BeforeSave() (err error) {
	err = policy.SerializeOptions()
	return err
}

//SerializeOptions 將序列後的Option寫入到資料庫欄位
func (policy *Policy) SerializeOptions() (err error) {
	optionsValue, err := json.Marshal(&policy.OptionsSerialized)
	policy.Options = string(optionsValue)
	return err
}

// GeneratePath 生成儲存文件的路徑
func (policy *Policy) GeneratePath(uid uint, origin string) string {
	dirRule := policy.DirNameRule
	replaceTable := map[string]string{
		"{randomkey16}":    util.RandStringRunes(16),
		"{randomkey8}":     util.RandStringRunes(8),
		"{timestamp}":      strconv.FormatInt(time.Now().Unix(), 10),
		"{timestamp_nano}": strconv.FormatInt(time.Now().UnixNano(), 10),
		"{uid}":            strconv.Itoa(int(uid)),
		"{datetime}":       time.Now().Format("20060102150405"),
		"{date}":           time.Now().Format("20060102"),
		"{year}":           time.Now().Format("2006"),
		"{month}":          time.Now().Format("01"),
		"{day}":            time.Now().Format("02"),
		"{hour}":           time.Now().Format("15"),
		"{minute}":         time.Now().Format("04"),
		"{second}":         time.Now().Format("05"),
		"{path}":           origin + "/",
	}
	dirRule = util.Replace(replaceTable, dirRule)
	return path.Clean(dirRule)
}

// GenerateFileName 生成儲存檔案名
func (policy *Policy) GenerateFileName(uid uint, origin string) string {
	// 未開啟自動重新命名時，直接返回原始檔案名
	if !policy.AutoRename {
		return policy.getOriginNameRule(origin)
	}

	fileRule := policy.FileNameRule

	replaceTable := map[string]string{
		"{randomkey16}":    util.RandStringRunes(16),
		"{randomkey8}":     util.RandStringRunes(8),
		"{timestamp}":      strconv.FormatInt(time.Now().Unix(), 10),
		"{timestamp_nano}": strconv.FormatInt(time.Now().UnixNano(), 10),
		"{uid}":            strconv.Itoa(int(uid)),
		"{datetime}":       time.Now().Format("20060102150405"),
		"{date}":           time.Now().Format("20060102"),
		"{year}":           time.Now().Format("2006"),
		"{month}":          time.Now().Format("01"),
		"{day}":            time.Now().Format("02"),
		"{hour}":           time.Now().Format("15"),
		"{minute}":         time.Now().Format("04"),
		"{second}":         time.Now().Format("05"),
	}

	replaceTable["{originname}"] = policy.getOriginNameRule(origin)

	fileRule = util.Replace(replaceTable, fileRule)
	return fileRule
}

func (policy Policy) getOriginNameRule(origin string) string {
	// 部分儲存策略可以使用{origin}代表原始檔案名
	if origin == "" {
		// 如果上游未傳回原始檔案名，則使用占位符，讓雲端儲存端取代
		switch policy.Type {
		case "qiniu":
			// 七牛會將$(fname)自動取代為原始檔案名
			return "$(fname)"
		case "local", "remote":
			return origin
		case "oss", "cos":
			// OSS會將${filename}自動取代為原始檔案名
			return "${filename}"
		case "upyun":
			// Upyun會將{filename}{.suffix}自動取代為原始檔案名
			return "{filename}{.suffix}"
		}
	}
	return origin
}

// IsDirectlyPreview 返回此策略下文件是否可以直接預覽（不需要重定向）
func (policy *Policy) IsDirectlyPreview() bool {
	return policy.Type == "local"
}

// IsThumbExist 給定檔案名，返回此儲存策略下是否可能存在縮圖
func (policy *Policy) IsThumbExist(name string) bool {
	if list, ok := thumbSuffix[policy.Type]; ok {
		if len(list) == 1 && list[0] == "*" {
			return true
		}
		return util.ContainsString(list, strings.ToLower(filepath.Ext(name)))
	}
	return false
}

// IsTransitUpload 返回此策略上傳給定size文件時是否需要服務端中轉
func (policy *Policy) IsTransitUpload(size uint64) bool {
	if policy.Type == "local" {
		return true
	}
	if policy.Type == "onedrive" && size < 4*1024*1024 {
		return true
	}
	return false
}

// IsPathGenerateNeeded 返回此策略是否需要在生成上傳憑證時生成儲存路徑
func (policy *Policy) IsPathGenerateNeeded() bool {
	return policy.Type != "remote"
}

// IsThumbGenerateNeeded 返回此策略是否需要在上傳後生成縮圖
func (policy *Policy) IsThumbGenerateNeeded() bool {
	return policy.Type == "local"
}

// CanStructureBeListed 返回儲存策略是否能被前台列物理目錄
func (policy *Policy) CanStructureBeListed() bool {
	return policy.Type != "local" && policy.Type != "remote"
}

// GetUploadURL 獲取文件上傳服務API地址
func (policy *Policy) GetUploadURL() string {
	server, err := url.Parse(policy.Server)
	if err != nil {
		return policy.Server
	}

	controller, _ := url.Parse("")
	switch policy.Type {
	case "local", "onedrive":
		return "/api/v3/file/upload"
	case "remote":
		controller, _ = url.Parse("/api/v3/slave/upload")
	case "oss":
		return "https://" + policy.BucketName + "." + policy.Server
	case "cos":
		return policy.Server
	case "upyun":
		return "https://v0.api.upyun.com/" + policy.BucketName
	case "s3":
		if policy.Server == "" {
			return fmt.Sprintf("https://%s.s3.%s.amazonaws.com/", policy.BucketName,
				policy.OptionsSerialized.Region)
		}

		if !strings.Contains(policy.Server, policy.BucketName) {
			controller, _ = url.Parse("/" + policy.BucketName)
		}
	}

	return server.ResolveReference(controller).String()
}

// SaveAndClearCache 更新並清理快取
func (policy *Policy) SaveAndClearCache() error {
	err := DB.Save(policy).Error
	policy.ClearCache()
	return err
}

// ClearCache 清空policy快取
func (policy *Policy) ClearCache() {
	cache.Deletes([]string{strconv.FormatUint(uint64(policy.ID), 10)}, "policy_")
}
