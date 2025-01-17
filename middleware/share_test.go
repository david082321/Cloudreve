package middleware

import (
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	model "github.com/cloudreve/Cloudreve/v3/models"
	"github.com/cloudreve/Cloudreve/v3/pkg/conf"
	"github.com/gin-gonic/gin"
	"github.com/jinzhu/gorm"
	"github.com/stretchr/testify/assert"
)

func TestShareAvailable(t *testing.T) {
	asserts := assert.New(t)
	rec := httptest.NewRecorder()
	testFunc := ShareAvailable()

	// 分享不存在
	{
		c, _ := gin.CreateTestContext(rec)
		c.Params = []gin.Param{
			{"id", "empty"},
		}
		testFunc(c)
		asserts.True(c.IsAborted())
	}

	// 通過
	{
		conf.SystemConfig.HashIDSalt = ""
		// 使用者群組
		mock.ExpectQuery("SELECT(.+)groups(.+)").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(3))
		mock.ExpectQuery("SELECT(.+)shares(.+)").
			WillReturnRows(
				sqlmock.NewRows(
					[]string{"id", "remain_downloads", "source_id"}).
					AddRow(1, 1, 2),
			)
		mock.ExpectQuery("SELECT(.+)files(.+)").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(2))
		c, _ := gin.CreateTestContext(rec)
		c.Params = []gin.Param{
			{"id", "x9T4"},
		}
		testFunc(c)
		asserts.NoError(mock.ExpectationsWereMet())
		asserts.False(c.IsAborted())
		asserts.NotNil(c.Get("user"))
		asserts.NotNil(c.Get("share"))
	}
}

func TestShareCanPreview(t *testing.T) {
	asserts := assert.New(t)
	rec := httptest.NewRecorder()
	testFunc := ShareCanPreview()

	// 無分享上下文
	{
		c, _ := gin.CreateTestContext(rec)
		testFunc(c)
		asserts.True(c.IsAborted())
	}

	// 可以預覽
	{
		c, _ := gin.CreateTestContext(rec)
		c.Set("share", &model.Share{PreviewEnabled: true})
		testFunc(c)
		asserts.False(c.IsAborted())
	}

	// 未開啟預覽
	{
		c, _ := gin.CreateTestContext(rec)
		c.Set("share", &model.Share{PreviewEnabled: false})
		testFunc(c)
		asserts.True(c.IsAborted())
	}
}

func TestCheckShareUnlocked(t *testing.T) {
	asserts := assert.New(t)
	rec := httptest.NewRecorder()
	testFunc := CheckShareUnlocked()

	// 無分享上下文
	{
		c, _ := gin.CreateTestContext(rec)
		testFunc(c)
		asserts.True(c.IsAborted())
	}

	// 無密碼
	{
		c, _ := gin.CreateTestContext(rec)
		c.Set("share", &model.Share{})
		testFunc(c)
		asserts.False(c.IsAborted())
	}

}

func TestBeforeShareDownload(t *testing.T) {
	asserts := assert.New(t)
	rec := httptest.NewRecorder()
	testFunc := BeforeShareDownload()

	// 無分享上下文
	{
		c, _ := gin.CreateTestContext(rec)
		testFunc(c)
		asserts.True(c.IsAborted())

		c, _ = gin.CreateTestContext(rec)
		c.Set("share", &model.Share{})
		testFunc(c)
		asserts.True(c.IsAborted())
	}

	// 使用者不能下載
	{
		c, _ := gin.CreateTestContext(rec)
		c.Set("share", &model.Share{})
		c.Set("user", &model.User{
			Group: model.Group{OptionsSerialized: model.GroupOption{}},
		})
		testFunc(c)
		asserts.True(c.IsAborted())
	}

	// 可以下載
	{
		c, _ := gin.CreateTestContext(rec)
		c.Set("share", &model.Share{})
		c.Set("user", &model.User{
			Model: gorm.Model{ID: 1},
			Group: model.Group{OptionsSerialized: model.GroupOption{
				ShareDownload: true,
			}},
		})
		testFunc(c)
		asserts.False(c.IsAborted())
	}
}

func TestShareOwner(t *testing.T) {
	asserts := assert.New(t)
	rec := httptest.NewRecorder()
	testFunc := ShareOwner()

	// 未登入
	{
		c, _ := gin.CreateTestContext(rec)
		testFunc(c)
		asserts.True(c.IsAborted())

		c, _ = gin.CreateTestContext(rec)
		c.Set("share", &model.Share{})
		testFunc(c)
		asserts.True(c.IsAborted())
	}

	// 非使用者所建立分享
	{
		c, _ := gin.CreateTestContext(rec)
		testFunc(c)
		asserts.True(c.IsAborted())

		c, _ = gin.CreateTestContext(rec)
		c.Set("share", &model.Share{User: model.User{Model: gorm.Model{ID: 1}}})
		c.Set("user", &model.User{})
		testFunc(c)
		asserts.True(c.IsAborted())
	}

	// 正常
	{
		c, _ := gin.CreateTestContext(rec)
		testFunc(c)
		asserts.True(c.IsAborted())

		c, _ = gin.CreateTestContext(rec)
		c.Set("share", &model.Share{})
		c.Set("user", &model.User{})
		testFunc(c)
		asserts.False(c.IsAborted())
	}
}
