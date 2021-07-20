package controllers

import "C"
import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"sync"

	model "github.com/cloudreve/Cloudreve/v3/models"
	"github.com/cloudreve/Cloudreve/v3/pkg/filesystem"
	"github.com/cloudreve/Cloudreve/v3/pkg/filesystem/driver/local"
	"github.com/cloudreve/Cloudreve/v3/pkg/filesystem/fsctx"
	"github.com/cloudreve/Cloudreve/v3/pkg/request"
	"github.com/cloudreve/Cloudreve/v3/pkg/serializer"
	"github.com/cloudreve/Cloudreve/v3/service/explorer"
	"github.com/gin-gonic/gin"
)

func DownloadArchive(c *gin.Context) {
	// 建立上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var service explorer.DownloadService
	if err := c.ShouldBindUri(&service); err == nil {
		res := service.DownloadArchived(ctx, c)
		if res.Code != 0 {
			c.JSON(200, res)
		}
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}

func Archive(c *gin.Context) {
	// 建立上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var service explorer.ItemIDService
	if err := c.ShouldBindJSON(&service); err == nil {
		res := service.Archive(ctx, c)
		c.JSON(200, res)
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}

// Compress 建立文件壓縮任務
func Compress(c *gin.Context) {
	var service explorer.ItemCompressService
	if err := c.ShouldBindJSON(&service); err == nil {
		res := service.CreateCompressTask(c)
		c.JSON(200, res)
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}

// Decompress 建立文件解壓縮任務
func Decompress(c *gin.Context) {
	var service explorer.ItemDecompressService
	if err := c.ShouldBindJSON(&service); err == nil {
		res := service.CreateDecompressTask(c)
		c.JSON(200, res)
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}

// AnonymousGetContent 匿名獲取文件資源
func AnonymousGetContent(c *gin.Context) {
	// 建立上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var service explorer.FileAnonymousGetService
	if err := c.ShouldBindUri(&service); err == nil {
		res := service.Download(ctx, c)
		if res.Code != 0 {
			c.JSON(200, res)
		}
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}

// AnonymousPermLink 文件簽名後的永久連結
func AnonymousPermLink(c *gin.Context) {
	// 建立上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var service explorer.FileAnonymousGetService
	if err := c.ShouldBindUri(&service); err == nil {
		res := service.Source(ctx, c)
		// 是否需要重定向
		if res.Code == -302 {
			c.Redirect(302, res.Data.(string))
			return
		}
		// 是否有錯誤發生
		if res.Code != 0 {
			c.JSON(200, res)
		}
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}

// GetSource 獲取文件的外鏈地址
func GetSource(c *gin.Context) {
	// 建立上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fs, err := filesystem.NewFileSystemFromContext(c)
	if err != nil {
		c.JSON(200, serializer.Err(serializer.CodePolicyNotAllowed, err.Error(), err))
		return
	}
	defer fs.Recycle()

	// 獲取文件ID
	fileID, ok := c.Get("object_id")
	if !ok {
		c.JSON(200, serializer.ParamErr("文件不存在", err))
		return
	}

	sourceURL, err := fs.GetSource(ctx, fileID.(uint))
	if err != nil {
		c.JSON(200, serializer.Err(serializer.CodeNotSet, err.Error(), err))
		return
	}

	c.JSON(200, serializer.Response{
		Code: 0,
		Data: struct {
			URL string `json:"url"`
		}{URL: sourceURL},
	})

}

// Thumb 獲取文件縮圖
func Thumb(c *gin.Context) {
	// 建立上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fs, err := filesystem.NewFileSystemFromContext(c)
	if err != nil {
		c.JSON(200, serializer.Err(serializer.CodePolicyNotAllowed, err.Error(), err))
		return
	}
	defer fs.Recycle()

	// 獲取文件ID
	fileID, ok := c.Get("object_id")
	if !ok {
		c.JSON(200, serializer.ParamErr("文件不存在", err))
		return
	}

	// 獲取縮圖
	resp, err := fs.GetThumb(ctx, fileID.(uint))
	if err != nil {
		c.JSON(200, serializer.Err(serializer.CodeNotSet, "無法獲取縮圖", err))
		return
	}

	if resp.Redirect {
		c.Header("Cache-Control", fmt.Sprintf("max-age=%d", resp.MaxAge))
		c.Redirect(http.StatusMovedPermanently, resp.URL)
		return
	}

	defer resp.Content.Close()
	http.ServeContent(c.Writer, c.Request, "thumb.png", fs.FileTarget[0].UpdatedAt, resp.Content)

}

// Preview 預覽文件
func Preview(c *gin.Context) {
	// 建立上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var service explorer.FileIDService
	if err := c.ShouldBindUri(&service); err == nil {
		res := service.PreviewContent(ctx, c, false)
		// 是否需要重定向
		if res.Code == -301 {
			c.Redirect(301, res.Data.(string))
			return
		}
		// 是否有錯誤發生
		if res.Code != 0 {
			c.JSON(200, res)
		}
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}

// PreviewText 預覽文字文件
func PreviewText(c *gin.Context) {
	// 建立上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var service explorer.FileIDService
	if err := c.ShouldBindUri(&service); err == nil {
		res := service.PreviewContent(ctx, c, true)
		// 是否有錯誤發生
		if res.Code != 0 {
			c.JSON(200, res)
		}
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}

// GetDocPreview 獲取DOC文件預覽地址
func GetDocPreview(c *gin.Context) {
	// 建立上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var service explorer.FileIDService
	if err := c.ShouldBindUri(&service); err == nil {
		res := service.CreateDocPreviewSession(ctx, c)
		c.JSON(200, res)
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}

// CreateDownloadSession 建立文件下載工作階段
func CreateDownloadSession(c *gin.Context) {
	// 建立上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var service explorer.FileIDService
	if err := c.ShouldBindUri(&service); err == nil {
		res := service.CreateDownloadSession(ctx, c)
		c.JSON(200, res)
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}

// Download 文件下載
func Download(c *gin.Context) {
	// 建立上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var service explorer.DownloadService
	if err := c.ShouldBindUri(&service); err == nil {
		res := service.Download(ctx, c)
		if res.Code != 0 {
			c.JSON(200, res)
		}
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}

// PutContent 更新文件內容
func PutContent(c *gin.Context) {
	// 建立上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var service explorer.FileIDService
	if err := c.ShouldBindUri(&service); err == nil {
		res := service.PutContent(ctx, c)
		c.JSON(200, res)
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}

// FileUploadStream 本機策略流式上傳
func FileUploadStream(c *gin.Context) {
	// 建立上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 取得檔案大小
	fileSize, err := strconv.ParseUint(c.Request.Header.Get("Content-Length"), 10, 64)
	if err != nil {
		c.JSON(200, ErrorResponse(err))
		return
	}

	// 非可用策略時拒絕上傳
	if user, ok := c.Get("user"); ok && !user.(*model.User).Policy.IsTransitUpload(fileSize) {
		request.BlackHole(c.Request.Body)
		c.JSON(200, serializer.Err(serializer.CodePolicyNotAllowed, "目前儲存策略無法使用", nil))
		return
	}

	// 解碼檔案名和路徑
	fileName, err := url.QueryUnescape(c.Request.Header.Get("X-FileName"))
	filePath, err := url.QueryUnescape(c.Request.Header.Get("X-Path"))
	if err != nil {
		c.JSON(200, ErrorResponse(err))
		return
	}

	fileData := local.FileStream{
		MIMEType:    c.Request.Header.Get("Content-Type"),
		File:        c.Request.Body,
		Size:        fileSize,
		Name:        fileName,
		VirtualPath: filePath,
	}

	// 建立文件系統
	fs, err := filesystem.NewFileSystemFromContext(c)
	if err != nil {
		c.JSON(200, serializer.Err(serializer.CodePolicyNotAllowed, err.Error(), err))
		return
	}

	// 給文件系統分配鉤子
	fs.Use("BeforeUpload", filesystem.HookValidateFile)
	fs.Use("BeforeUpload", filesystem.HookValidateCapacity)
	fs.Use("AfterUploadCanceled", filesystem.HookDeleteTempFile)
	fs.Use("AfterUploadCanceled", filesystem.HookGiveBackCapacity)
	fs.Use("AfterUpload", filesystem.GenericAfterUpload)
	fs.Use("AfterValidateFailed", filesystem.HookDeleteTempFile)
	fs.Use("AfterValidateFailed", filesystem.HookGiveBackCapacity)
	fs.Use("AfterUploadFailed", filesystem.HookGiveBackCapacity)

	// 執行上傳
	ctx = context.WithValue(ctx, fsctx.ValidateCapacityOnceCtx, &sync.Once{})
	ctx = context.WithValue(ctx, fsctx.DisableOverwrite, true)
	uploadCtx := context.WithValue(ctx, fsctx.GinCtx, c)
	err = fs.Upload(uploadCtx, fileData)
	if err != nil {
		c.JSON(200, serializer.Err(serializer.CodeUploadFailed, err.Error(), err))
		return
	}

	c.JSON(200, serializer.Response{
		Code: 0,
	})
}

// GetUploadCredential 獲取上傳憑證
func GetUploadCredential(c *gin.Context) {
	// 建立上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var service explorer.UploadCredentialService
	if err := c.ShouldBindQuery(&service); err == nil {
		res := service.Get(ctx, c)
		c.JSON(200, res)
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}

// SearchFile 搜尋文件
func SearchFile(c *gin.Context) {
	var service explorer.ItemSearchService
	if err := c.ShouldBindUri(&service); err == nil {
		res := service.Search(c)
		c.JSON(200, res)
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}

// CreateFile 建立空白文件
func CreateFile(c *gin.Context) {
	var service explorer.SingleFileService
	if err := c.ShouldBindJSON(&service); err == nil {
		res := service.Create(c)
		c.JSON(200, res)
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}
