package middleware

import (
	"github.com/cloudreve/Cloudreve/v3/pkg/conf"
	"github.com/cloudreve/Cloudreve/v3/pkg/serializer"
	"github.com/cloudreve/Cloudreve/v3/pkg/util"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/memstore"
	"github.com/gin-contrib/sessions/redis"
	"github.com/gin-gonic/gin"
)

// Store session儲存
var Store memstore.Store

// Session 初始化session
func Session(secret string) gin.HandlerFunc {
	// Redis設定不為空，且非測試模式時使用Redis
	if conf.RedisConfig.Server != "" && gin.Mode() != gin.TestMode {
		var err error
		Store, err = redis.NewStoreWithDB(10, conf.RedisConfig.Network, conf.RedisConfig.Server, conf.RedisConfig.Password, conf.RedisConfig.DB, []byte(secret))
		if err != nil {
			util.Log().Panic("無法連接到 Redis：%s", err)
		}

		util.Log().Info("已連接到 Redis 伺服器：%s", conf.RedisConfig.Server)
	} else {
		Store = memstore.NewStore([]byte(secret))
	}

	// Also set Secure: true if using SSL, you should though
	// TODO:same-site policy
	Store.Options(sessions.Options{HttpOnly: true, MaxAge: 7 * 86400, Path: "/"})
	return sessions.Sessions("cloudreve-session", Store)
}

// CSRFInit 初始化CSRF標記
func CSRFInit() gin.HandlerFunc {
	return func(c *gin.Context) {
		util.SetSession(c, map[string]interface{}{"CSRF": true})
		c.Next()
	}
}

// CSRFCheck 檢查CSRF標記
func CSRFCheck() gin.HandlerFunc {
	return func(c *gin.Context) {
		if check, ok := util.GetSession(c, "CSRF").(bool); ok && check {
			c.Next()
			return
		}

		c.JSON(200, serializer.Err(serializer.CodeNoPermissionErr, "來源非法", nil))
		c.Abort()
	}
}
