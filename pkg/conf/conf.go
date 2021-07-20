package conf

import (
	"github.com/cloudreve/Cloudreve/v3/pkg/util"
	"github.com/go-ini/ini"
	"gopkg.in/go-playground/validator.v9"
)

// database 資料庫
type database struct {
	Type        string
	User        string
	Password    string
	Host        string
	Name        string
	TablePrefix string
	DBFile      string
	Port        int
	Charset     string
}

// system 系統通用配置
type system struct {
	Mode          string `validate:"eq=master|eq=slave"`
	Listen        string `validate:"required"`
	Debug         bool
	SessionSecret string
	HashIDSalt    string
}

type ssl struct {
	CertPath string `validate:"omitempty,required"`
	KeyPath  string `validate:"omitempty,required"`
	Listen   string `validate:"required"`
}

type unix struct {
	Listen string
}

// slave 作為slave儲存端配置
type slave struct {
	Secret          string `validate:"omitempty,gte=64"`
	CallbackTimeout int    `validate:"omitempty,gte=1"`
	SignatureTTL    int    `validate:"omitempty,gte=1"`
}

// captcha 驗證碼配置
type captcha struct {
	Height             int `validate:"gte=0"`
	Width              int `validate:"gte=0"`
	Mode               int `validate:"gte=0,lte=3"`
	ComplexOfNoiseText int `validate:"gte=0,lte=2"`
	ComplexOfNoiseDot  int `validate:"gte=0,lte=2"`
	IsShowHollowLine   bool
	IsShowNoiseDot     bool
	IsShowNoiseText    bool
	IsShowSlimeLine    bool
	IsShowSineLine     bool
	CaptchaLen         int `validate:"gt=0"`
}

// redis 配置
type redis struct {
	Network  string
	Server   string
	Password string
	DB       string
}

// 縮圖 配置
type thumb struct {
	MaxWidth   uint
	MaxHeight  uint
	FileSuffix string `validate:"min=1"`
}

// 跨域配置
type cors struct {
	AllowOrigins     []string
	AllowMethods     []string
	AllowHeaders     []string
	AllowCredentials bool
	ExposeHeaders    []string
}

var cfg *ini.File

const defaultConf = `[System]
Mode = master
Listen = :5212
SessionSecret = {SessionSecret}
HashIDSalt = {HashIDSalt}
`

// Init 初始化配置檔案
func Init(path string) {
	var err error

	if path == "" || !util.Exists(path) {
		// 建立初始配置檔案
		confContent := util.Replace(map[string]string{
			"{SessionSecret}": util.RandStringRunes(64),
			"{HashIDSalt}":    util.RandStringRunes(64),
		}, defaultConf)
		f, err := util.CreatNestedFile(path)
		if err != nil {
			util.Log().Panic("無法建立配置檔案, %s", err)
		}

		// 寫入配置檔案
		_, err = f.WriteString(confContent)
		if err != nil {
			util.Log().Panic("無法寫入配置檔案, %s", err)
		}

		f.Close()
	}

	cfg, err = ini.Load(path)
	if err != nil {
		util.Log().Panic("無法解析配置檔案 '%s': %s", path, err)
	}

	sections := map[string]interface{}{
		"Database":   DatabaseConfig,
		"System":     SystemConfig,
		"SSL":        SSLConfig,
		"UnixSocket": UnixConfig,
		"Captcha":    CaptchaConfig,
		"Redis":      RedisConfig,
		"Thumbnail":  ThumbConfig,
		"CORS":       CORSConfig,
		"Slave":      SlaveConfig,
	}
	for sectionName, sectionStruct := range sections {
		err = mapSection(sectionName, sectionStruct)
		if err != nil {
			util.Log().Panic("配置檔案 %s 分區解析失敗: %s", sectionName, err)
		}
	}

	// 重設log等級
	if !SystemConfig.Debug {
		util.Level = util.LevelInformational
		util.GloablLogger = nil
		util.Log()
	}

}

// mapSection 將配置檔案的 Section 映射到結構體上
func mapSection(section string, confStruct interface{}) error {
	err := cfg.Section(section).MapTo(confStruct)
	if err != nil {
		return err
	}

	// 驗證合法性
	validate := validator.New()
	err = validate.Struct(confStruct)
	if err != nil {
		return err
	}

	return nil
}
