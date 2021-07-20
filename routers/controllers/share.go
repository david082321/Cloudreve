package controllers

import (
	"context"
	"path"
	"strings"

	model "github.com/cloudreve/Cloudreve/v3/models"
	"github.com/cloudreve/Cloudreve/v3/pkg/serializer"
	"github.com/cloudreve/Cloudreve/v3/pkg/util"
	"github.com/cloudreve/Cloudreve/v3/service/share"
	"github.com/gin-gonic/gin"
)

// CreateShare 建立分享
func CreateShare(c *gin.Context) {
	var service share.ShareCreateService
	if err := c.ShouldBindJSON(&service); err == nil {
		res := service.Create(c)
		c.JSON(200, res)
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}

// GetShare 查看分享
func GetShare(c *gin.Context) {
	var service share.ShareGetService
	if err := c.ShouldBindQuery(&service); err == nil {
		res := service.Get(c)
		c.JSON(200, res)
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}

// ListShare 列出分享
func ListShare(c *gin.Context) {
	var service share.ShareListService
	if err := c.ShouldBindQuery(&service); err == nil {
		res := service.List(c, CurrentUser(c))
		c.JSON(200, res)
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}

// SearchShare 搜尋分享
func SearchShare(c *gin.Context) {
	var service share.ShareListService
	if err := c.ShouldBindQuery(&service); err == nil {
		res := service.Search(c)
		c.JSON(200, res)
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}

// UpdateShare 更新分享屬性
func UpdateShare(c *gin.Context) {
	var service share.ShareUpdateService
	if err := c.ShouldBindJSON(&service); err == nil {
		res := service.Update(c)
		c.JSON(200, res)
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}

// DeleteShare 刪除分享
func DeleteShare(c *gin.Context) {
	var service share.Service
	if err := c.ShouldBindUri(&service); err == nil {
		res := service.Delete(c, CurrentUser(c))
		c.JSON(200, res)
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}

// GetShareDownload 建立分享下載工作階段
func GetShareDownload(c *gin.Context) {
	var service share.Service
	if err := c.ShouldBindQuery(&service); err == nil {
		res := service.CreateDownloadSession(c)
		c.JSON(200, res)
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}

// PreviewShare 預覽分享文件內容
func PreviewShare(c *gin.Context) {
	// 建立上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var service share.Service
	if err := c.ShouldBindQuery(&service); err == nil {
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

// PreviewShareText 預覽文字文件
func PreviewShareText(c *gin.Context) {
	// 建立上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var service share.Service
	if err := c.ShouldBindQuery(&service); err == nil {
		res := service.PreviewContent(ctx, c, true)
		// 是否有錯誤發生
		if res.Code != 0 {
			c.JSON(200, res)
		}
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}

// PreviewShareReadme 預覽文字自述文件
func PreviewShareReadme(c *gin.Context) {
	// 建立上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var service share.Service
	if err := c.ShouldBindQuery(&service); err == nil {
		// 自述檔案名限制
		allowFileName := []string{"readme.txt", "readme.md"}
		fileName := strings.ToLower(path.Base(service.Path))
		if !util.ContainsString(allowFileName, fileName) {
			c.JSON(200, serializer.ParamErr("非README文件", nil))
		}

		// 必須是目錄分享
		if shareCtx, ok := c.Get("share"); ok {
			if !shareCtx.(*model.Share).IsDir {
				c.JSON(200, serializer.ParamErr("此分享無自述文件", nil))
			}
		}

		res := service.PreviewContent(ctx, c, true)
		// 是否有錯誤發生
		if res.Code != 0 {
			c.JSON(200, res)
		}
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}

// GetShareDocPreview 建立分享Office文件預覽地址
func GetShareDocPreview(c *gin.Context) {
	var service share.Service
	if err := c.ShouldBindQuery(&service); err == nil {
		res := service.CreateDocPreviewSession(c)
		c.JSON(200, res)
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}

// ListSharedFolder 列出分享的目錄下的物件
func ListSharedFolder(c *gin.Context) {
	var service share.Service
	if err := c.ShouldBindUri(&service); err == nil {
		res := service.List(c)
		c.JSON(200, res)
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}

// ArchiveShare 打包要下載的分享
func ArchiveShare(c *gin.Context) {
	var service share.ArchiveService
	if err := c.ShouldBindJSON(&service); err == nil {
		res := service.Archive(c)
		c.JSON(200, res)
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}

// ShareThumb 獲取分享目錄下文件的縮圖
func ShareThumb(c *gin.Context) {
	var service share.Service
	if err := c.ShouldBindQuery(&service); err == nil {
		res := service.Thumb(c)
		if res.Code >= 0 {
			c.JSON(200, res)
		}
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}

// GetUserShare 查看給定使用者的分享
func GetUserShare(c *gin.Context) {
	var service share.ShareUserGetService
	if err := c.ShouldBindQuery(&service); err == nil {
		res := service.Get(c)
		c.JSON(200, res)
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}
