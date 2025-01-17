package controllers

import (
	"github.com/cloudreve/Cloudreve/v3/service/explorer"
	"github.com/gin-gonic/gin"
)

// CreateDirectory 建立目錄
func CreateDirectory(c *gin.Context) {
	var service explorer.DirectoryService
	if err := c.ShouldBindJSON(&service); err == nil {
		res := service.CreateDirectory(c)
		c.JSON(200, res)
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}

// ListDirectory 列出目錄下內容
func ListDirectory(c *gin.Context) {
	var service explorer.DirectoryService
	if err := c.ShouldBindUri(&service); err == nil {
		res := service.ListDirectory(c)
		c.JSON(200, res)
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}
