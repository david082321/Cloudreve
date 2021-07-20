package auth

import (
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	model "github.com/cloudreve/Cloudreve/v3/models"
	"github.com/cloudreve/Cloudreve/v3/pkg/conf"
	"github.com/cloudreve/Cloudreve/v3/pkg/util"
	"github.com/gin-gonic/gin"
	"github.com/jinzhu/gorm"
	"github.com/stretchr/testify/assert"
)

var mock sqlmock.Sqlmock

func TestMain(m *testing.M) {
	// 設定gin為測試模式
	gin.SetMode(gin.TestMode)

	// 初始化sqlmock
	var db *sql.DB
	var err error
	db, mock, err = sqlmock.New()
	if err != nil {
		panic("An error was not expected when opening a stub database connection")
	}

	mockDB, _ := gorm.Open("mysql", db)
	model.DB = mockDB
	defer db.Close()

	m.Run()
}

func TestHMACAuth_Sign(t *testing.T) {
	asserts := assert.New(t)
	auth := HMACAuth{
		SecretKey: []byte(util.RandStringRunes(256)),
	}

	asserts.NotEmpty(auth.Sign("content", 0))
}

func TestHMACAuth_Check(t *testing.T) {
	asserts := assert.New(t)
	auth := HMACAuth{
		SecretKey: []byte(util.RandStringRunes(256)),
	}

	// 正常，永不過期
	{
		sign := auth.Sign("content", 0)
		asserts.NoError(auth.Check("content", sign))
	}

	// 過期
	{
		sign := auth.Sign("content", 1)
		asserts.Error(auth.Check("content", sign))
	}

	// 簽名格式錯誤
	{
		sign := auth.Sign("content", 1)
		asserts.Error(auth.Check("content", sign+":"))
	}

	// 過期日期格式錯誤
	{
		asserts.Error(auth.Check("content", "ErrAuthFailed:ErrAuthFailed"))
	}

	// 簽名有誤
	{
		asserts.Error(auth.Check("content", fmt.Sprintf("sign:%d", time.Now().Unix()+10)))
	}
}

func TestInit(t *testing.T) {
	asserts := assert.New(t)
	mock.ExpectQuery("SELECT(.+)").WillReturnRows(sqlmock.NewRows([]string{"id", "value"}).AddRow(1, "12312312312312"))
	Init()
	asserts.NoError(mock.ExpectationsWereMet())

	// slave模式
	conf.SystemConfig.Mode = "slave"
	asserts.Panics(func() {
		Init()
	})
}
