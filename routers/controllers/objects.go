package controllers

import (
	"context"

	"github.com/cloudreve/Cloudreve/v3/service/explorer"
	"github.com/gin-gonic/gin"
)

// Delete 刪除文件或目錄
func Delete(c *gin.Context) {
	// 建立上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var service explorer.ItemIDService
	if err := c.ShouldBindJSON(&service); err == nil {
		res := service.Delete(ctx, c)
		c.JSON(200, res)
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}

// Move 移動文件或目錄
func Move(c *gin.Context) {
	// 建立上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var service explorer.ItemMoveService
	if err := c.ShouldBindJSON(&service); err == nil {
		res := service.Move(ctx, c)
		c.JSON(200, res)
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}

// Copy 複製文件或目錄
func Copy(c *gin.Context) {
	// 建立上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var service explorer.ItemMoveService
	if err := c.ShouldBindJSON(&service); err == nil {
		res := service.Copy(ctx, c)
		c.JSON(200, res)
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}

// Rename 重新命名文件或目錄
func Rename(c *gin.Context) {
	// 建立上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var service explorer.ItemRenameService
	if err := c.ShouldBindJSON(&service); err == nil {
		res := service.Rename(ctx, c)
		c.JSON(200, res)
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}

// Rename 重新命名文件或目錄
func GetProperty(c *gin.Context) {
	// 建立上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var service explorer.ItemPropertyService
	service.ID = c.Param("id")
	if err := c.ShouldBindQuery(&service); err == nil {
		res := service.GetProperty(ctx, c)
		c.JSON(200, res)
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}
