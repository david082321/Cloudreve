package controllers

import (
	"github.com/cloudreve/Cloudreve/v3/service/explorer"
	"github.com/gin-gonic/gin"
)

// CreateFilterTag 建立文件分類標籤
func CreateFilterTag(c *gin.Context) {
	var service explorer.FilterTagCreateService
	if err := c.ShouldBindJSON(&service); err == nil {
		res := service.Create(c, CurrentUser(c))
		c.JSON(200, res)
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}

// CreateLinkTag 建立目錄捷徑標籤
func CreateLinkTag(c *gin.Context) {
	var service explorer.LinkTagCreateService
	if err := c.ShouldBindJSON(&service); err == nil {
		res := service.Create(c, CurrentUser(c))
		c.JSON(200, res)
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}

// DeleteTag 刪除標籤
func DeleteTag(c *gin.Context) {
	var service explorer.TagService
	if err := c.ShouldBindUri(&service); err == nil {
		res := service.Delete(c, CurrentUser(c))
		c.JSON(200, res)
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}
