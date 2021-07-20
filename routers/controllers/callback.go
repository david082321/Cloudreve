package controllers

import (
	model "github.com/cloudreve/Cloudreve/v3/models"
	"path"
	"strconv"

	"github.com/cloudreve/Cloudreve/v3/pkg/serializer"
	"github.com/cloudreve/Cloudreve/v3/pkg/util"
	"github.com/cloudreve/Cloudreve/v3/service/callback"
	"github.com/gin-gonic/gin"
)

// RemoteCallback 遠端上傳回調
func RemoteCallback(c *gin.Context) {
	var callbackBody callback.RemoteUploadCallbackService
	if err := c.ShouldBindJSON(&callbackBody); err == nil {
		res := callback.ProcessCallback(callbackBody, c)
		c.JSON(200, res)
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}

// QiniuCallback 七牛上傳回調
func QiniuCallback(c *gin.Context) {
	var callbackBody callback.UploadCallbackService
	if err := c.ShouldBindJSON(&callbackBody); err == nil {
		res := callback.ProcessCallback(callbackBody, c)
		if res.Code != 0 {
			c.JSON(401, serializer.GeneralUploadCallbackFailed{Error: res.Msg})
		} else {
			c.JSON(200, res)
		}
	} else {
		c.JSON(401, ErrorResponse(err))
	}
}

// OSSCallback 阿里雲OSS上傳回調
func OSSCallback(c *gin.Context) {
	var callbackBody callback.UploadCallbackService
	if err := c.ShouldBindJSON(&callbackBody); err == nil {
		if callbackBody.PicInfo == "," {
			callbackBody.PicInfo = ""
		}
		res := callback.ProcessCallback(callbackBody, c)
		c.JSON(200, res)
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}

// UpyunCallback 又拍雲上傳回調
func UpyunCallback(c *gin.Context) {
	var callbackBody callback.UpyunCallbackService
	if err := c.ShouldBind(&callbackBody); err == nil {
		if callbackBody.Code != 200 {
			util.Log().Debug(
				"又拍雲回調返回錯誤代碼%d，訊息：%s",
				callbackBody.Code,
				callbackBody.Message,
			)
			return
		}
		res := callback.ProcessCallback(callbackBody, c)
		c.JSON(200, res)
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}

// OneDriveCallback OneDrive上傳完成用戶端回調
func OneDriveCallback(c *gin.Context) {
	var callbackBody callback.OneDriveCallback
	if err := c.ShouldBindJSON(&callbackBody); err == nil {
		res := callbackBody.PreProcess(c)
		c.JSON(200, res)
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}

// OneDriveOAuth OneDrive 授權回調
func OneDriveOAuth(c *gin.Context) {
	var callbackBody callback.OneDriveOauthService
	if err := c.ShouldBindQuery(&callbackBody); err == nil {
		res := callbackBody.Auth(c)
		redirect := model.GetSiteURL()
		redirect.Path = path.Join(redirect.Path, "/admin/policy")
		queries := redirect.Query()
		queries.Add("code", strconv.Itoa(res.Code))
		queries.Add("msg", res.Msg)
		queries.Add("err", res.Error)
		redirect.RawQuery = queries.Encode()
		c.Redirect(303, redirect.String())
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}

// COSCallback COS上傳完成用戶端回調
func COSCallback(c *gin.Context) {
	var callbackBody callback.COSCallback
	if err := c.ShouldBindQuery(&callbackBody); err == nil {
		res := callbackBody.PreProcess(c)
		c.JSON(200, res)
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}

// S3Callback S3上傳完成用戶端回調
func S3Callback(c *gin.Context) {
	c.Header("Access-Control-Allow-Origin", "*")
	var callbackBody callback.S3Callback
	if err := c.ShouldBindQuery(&callbackBody); err == nil {
		res := callbackBody.PreProcess(c)
		c.JSON(200, res)
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}
