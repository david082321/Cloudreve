package model

import (
	"fmt"
	"time"

	"github.com/cloudreve/Cloudreve/v3/pkg/conf"
	"github.com/cloudreve/Cloudreve/v3/pkg/util"
	"github.com/gin-gonic/gin"
	"github.com/jinzhu/gorm"

	_ "github.com/jinzhu/gorm/dialects/mssql"
	_ "github.com/jinzhu/gorm/dialects/mysql"
	_ "github.com/jinzhu/gorm/dialects/postgres"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
)

// DB 資料庫連結單例
var DB *gorm.DB

// Init 初始化 MySQL 連結
func Init() {
	util.Log().Info("初始化資料庫連接")

	var (
		db  *gorm.DB
		err error
	)

	if gin.Mode() == gin.TestMode {
		// 測試模式下，使用記憶體資料庫
		db, err = gorm.Open("sqlite3", ":memory:")
	} else {
		switch conf.DatabaseConfig.Type {
		case "UNSET", "sqlite", "sqlite3":
			// 未指定資料庫或者明確指定為 sqlite 時，使用 SQLite3 資料庫
			db, err = gorm.Open("sqlite3", util.RelativePath(conf.DatabaseConfig.DBFile))
		case "mysql", "postgres", "mssql":
			db, err = gorm.Open(conf.DatabaseConfig.Type, fmt.Sprintf("%s:%s@(%s:%d)/%s?charset=%s&parseTime=True&loc=Local",
				conf.DatabaseConfig.User,
				conf.DatabaseConfig.Password,
				conf.DatabaseConfig.Host,
				conf.DatabaseConfig.Port,
				conf.DatabaseConfig.Name,
				conf.DatabaseConfig.Charset))
		default:
			util.Log().Panic("不支援資料庫類型: %s", conf.DatabaseConfig.Type)
		}
	}

	//db.SetLogger(util.Log())
	if err != nil {
		util.Log().Panic("連接資料庫不成功, %s", err)
	}

	// 處理表前綴
	gorm.DefaultTableNameHandler = func(db *gorm.DB, defaultTableName string) string {
		return conf.DatabaseConfig.TablePrefix + defaultTableName
	}

	// Debug模式下，輸出所有 SQL 日誌
	if conf.SystemConfig.Debug {
		db.LogMode(true)
	} else {
		db.LogMode(false)
	}

	//設定連接池
	//空閒
	db.DB().SetMaxIdleConns(50)
	//打開
	db.DB().SetMaxOpenConns(100)
	//超時
	db.DB().SetConnMaxLifetime(time.Second * 30)

	DB = db

	//執行遷移
	migration()
}
