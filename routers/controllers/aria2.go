package controllers

import (
	"context"

	ariaCall "github.com/cloudreve/Cloudreve/v3/pkg/aria2"
	"github.com/cloudreve/Cloudreve/v3/service/aria2"
	"github.com/cloudreve/Cloudreve/v3/service/explorer"
	"github.com/gin-gonic/gin"
)

// AddAria2URL 添加離線下載URL
func AddAria2URL(c *gin.Context) {
	var addService aria2.AddURLService
	if err := c.ShouldBindJSON(&addService); err == nil {
		res := addService.Add(c, ariaCall.URLTask)
		c.JSON(200, res)
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}

// SelectAria2File 選擇多文件離線下載中要下載的文件
func SelectAria2File(c *gin.Context) {
	var selectService aria2.SelectFileService
	if err := c.ShouldBindJSON(&selectService); err == nil {
		res := selectService.Select(c)
		c.JSON(200, res)
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}

// AddAria2Torrent 添加離線下載種子
func AddAria2Torrent(c *gin.Context) {
	// 建立上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var service explorer.FileIDService
	if err := c.ShouldBindUri(&service); err == nil {
		// 獲取種子內容的下載網址
		res := service.CreateDownloadSession(ctx, c)
		if res.Code != 0 {
			c.JSON(200, res)
			return
		}

		// 建立下載任務
		var addService aria2.AddURLService
		addService.URL = res.Data.(string)

		if err := c.ShouldBindJSON(&addService); err == nil {
			addService.URL = res.Data.(string)
			res := addService.Add(c, ariaCall.URLTask)
			c.JSON(200, res)
		} else {
			c.JSON(200, ErrorResponse(err))
		}

	} else {
		c.JSON(200, ErrorResponse(err))
	}
}

// CancelAria2Download 取消或刪除aria2離線下載任務
func CancelAria2Download(c *gin.Context) {
	var selectService aria2.DownloadTaskService
	if err := c.ShouldBindUri(&selectService); err == nil {
		res := selectService.Delete(c)
		c.JSON(200, res)
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}

// ListDownloading 獲取正在下載中的任務
func ListDownloading(c *gin.Context) {
	var service aria2.DownloadListService
	if err := c.ShouldBindQuery(&service); err == nil {
		res := service.Downloading(c, CurrentUser(c))
		c.JSON(200, res)
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}

// ListFinished 獲取已完成的任務
func ListFinished(c *gin.Context) {
	var service aria2.DownloadListService
	if err := c.ShouldBindQuery(&service); err == nil {
		res := service.Finished(c, CurrentUser(c))
		c.JSON(200, res)
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}
