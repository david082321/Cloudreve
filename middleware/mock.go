package middleware

import (
	"github.com/cloudreve/Cloudreve/v3/pkg/util"
	"github.com/gin-gonic/gin"
)

// SessionMock 測試時模擬Session
var SessionMock = make(map[string]interface{})

// ContextMock 測試時模擬Context
var ContextMock = make(map[string]interface{})

// MockHelper 單元測試助手中間件
func MockHelper() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 將SessionMock寫入工作階段
		util.SetSession(c, SessionMock)
		for key, value := range ContextMock {
			c.Set(key, value)
		}
		c.Next()
	}
}
