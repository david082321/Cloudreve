package controllers

import (
	"context"
	"net/url"
	"strconv"

	"github.com/cloudreve/Cloudreve/v3/pkg/filesystem"
	"github.com/cloudreve/Cloudreve/v3/pkg/filesystem/driver/local"
	"github.com/cloudreve/Cloudreve/v3/pkg/filesystem/fsctx"
	"github.com/cloudreve/Cloudreve/v3/pkg/serializer"
	"github.com/cloudreve/Cloudreve/v3/service/admin"
	"github.com/cloudreve/Cloudreve/v3/service/explorer"
	"github.com/gin-gonic/gin"
)

// SlaveUpload 從機文件上傳
func SlaveUpload(c *gin.Context) {
	// 建立上下文
	ctx, cancel := context.WithCancel(context.Background())
	ctx = context.WithValue(ctx, fsctx.GinCtx, c)
	defer cancel()

	// 建立匿名文件系統
	fs, err := filesystem.NewAnonymousFileSystem()
	if err != nil {
		c.JSON(200, serializer.Err(serializer.CodePolicyNotAllowed, err.Error(), err))
		return
	}
	fs.Handler = local.Driver{}

	// 從請求中取得上傳策略
	uploadPolicyRaw := c.GetHeader("X-Policy")
	if uploadPolicyRaw == "" {
		c.JSON(200, serializer.ParamErr("未指定上傳策略", nil))
		return
	}

	// 解析上傳策略
	uploadPolicy, err := serializer.DecodeUploadPolicy(uploadPolicyRaw)
	if err != nil {
		c.JSON(200, serializer.ParamErr("上傳策略格式有誤", err))
		return
	}
	ctx = context.WithValue(ctx, fsctx.UploadPolicyCtx, *uploadPolicy)

	// 取得檔案大小
	fileSize, err := strconv.ParseUint(c.Request.Header.Get("Content-Length"), 10, 64)
	if err != nil {
		c.JSON(200, ErrorResponse(err))
		return
	}

	// 解碼檔案名和路徑
	fileName, err := url.QueryUnescape(c.Request.Header.Get("X-FileName"))
	if err != nil {
		c.JSON(200, ErrorResponse(err))
		return
	}

	fileData := local.FileStream{
		MIMEType: c.Request.Header.Get("Content-Type"),
		File:     c.Request.Body,
		Name:     fileName,
		Size:     fileSize,
	}

	// 給文件系統分配鉤子
	fs.Use("BeforeUpload", filesystem.HookSlaveUploadValidate)
	fs.Use("AfterUploadCanceled", filesystem.HookDeleteTempFile)
	fs.Use("AfterUpload", filesystem.SlaveAfterUpload)
	fs.Use("AfterValidateFailed", filesystem.HookDeleteTempFile)

	// 是否允許覆蓋
	if c.Request.Header.Get("X-Overwrite") == "false" {
		ctx = context.WithValue(ctx, fsctx.DisableOverwrite, true)
	}

	// 執行上傳
	err = fs.Upload(ctx, fileData)
	if err != nil {
		c.JSON(200, serializer.Err(serializer.CodeUploadFailed, err.Error(), err))
		return
	}

	c.JSON(200, serializer.Response{
		Code: 0,
	})
}

// SlaveDownload 從機文件下載,此請求返回的HTTP狀態碼不全為200
func SlaveDownload(c *gin.Context) {
	// 建立上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var service explorer.SlaveDownloadService
	if err := c.ShouldBindUri(&service); err == nil {
		res := service.ServeFile(ctx, c, true)
		if res.Code != 0 {
			c.JSON(400, res)
		}
	} else {
		c.JSON(400, ErrorResponse(err))
	}
}

// SlavePreview 從機文件預覽
func SlavePreview(c *gin.Context) {
	// 建立上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var service explorer.SlaveDownloadService
	if err := c.ShouldBindUri(&service); err == nil {
		res := service.ServeFile(ctx, c, false)
		if res.Code != 0 {
			c.JSON(200, res)
		}
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}

// SlaveThumb 從機文件縮圖
func SlaveThumb(c *gin.Context) {
	// 建立上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var service explorer.SlaveFileService
	if err := c.ShouldBindUri(&service); err == nil {
		res := service.Thumb(ctx, c)
		if res.Code != 0 {
			c.JSON(200, res)
		}
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}

// SlaveDelete 從機刪除
func SlaveDelete(c *gin.Context) {
	// 建立上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var service explorer.SlaveFilesService
	if err := c.ShouldBindJSON(&service); err == nil {
		res := service.Delete(ctx, c)
		c.JSON(200, res)
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}

// SlavePing 從機測試
func SlavePing(c *gin.Context) {
	var service admin.SlavePingService
	if err := c.ShouldBindJSON(&service); err == nil {
		res := service.Test()
		c.JSON(200, res)
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}

// SlaveList 從機列出文件
func SlaveList(c *gin.Context) {
	var service explorer.SlaveListService
	if err := c.ShouldBindJSON(&service); err == nil {
		res := service.List(c)
		c.JSON(200, res)
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}
