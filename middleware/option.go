package middleware

import (
	model "github.com/cloudreve/Cloudreve/v3/models"
	"github.com/cloudreve/Cloudreve/v3/pkg/hashid"
	"github.com/cloudreve/Cloudreve/v3/pkg/serializer"
	"github.com/gin-gonic/gin"
)

// HashID 將給定物件的HashID轉換為真實ID
func HashID(IDType int) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Param("id") != "" {
			id, err := hashid.DecodeHashID(c.Param("id"), IDType)
			if err == nil {
				c.Set("object_id", id)
				c.Next()
				return
			}
			c.JSON(200, serializer.ParamErr("無法解析物件ID", nil))
			c.Abort()
			return

		}
		c.Next()
	}
}

// IsFunctionEnabled 當功能未開啟時阻止訪問
func IsFunctionEnabled(key string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !model.IsTrueVal(model.GetSettingByName(key)) {
			c.JSON(200, serializer.Err(serializer.CodeNoPermissionErr, "未開啟此功能", nil))
			c.Abort()
			return
		}

		c.Next()
	}
}
