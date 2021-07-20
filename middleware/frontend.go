package middleware

import (
	"github.com/cloudreve/Cloudreve/v3/bootstrap"
	model "github.com/cloudreve/Cloudreve/v3/models"
	"github.com/cloudreve/Cloudreve/v3/pkg/util"
	"github.com/gin-gonic/gin"
	"io/ioutil"
	"net/http"
	"strings"
)

// FrontendFileHandler 前端靜態文件處理
func FrontendFileHandler() gin.HandlerFunc {
	ignoreFunc := func(c *gin.Context) {
		c.Next()
	}

	if bootstrap.StaticFS == nil {
		return ignoreFunc
	}

	// 讀取index.html
	file, err := bootstrap.StaticFS.Open("/index.html")
	if err != nil {
		util.Log().Warning("靜態文件[index.html]不存在，可能會影響首頁展示")
		return ignoreFunc
	}

	fileContentBytes, err := ioutil.ReadAll(file)
	if err != nil {
		util.Log().Warning("靜態文件[index.html]讀取失敗，可能會影響首頁展示")
		return ignoreFunc
	}
	fileContent := string(fileContentBytes)

	fileServer := http.FileServer(bootstrap.StaticFS)
	return func(c *gin.Context) {
		path := c.Request.URL.Path

		// API 跳過
		if strings.HasPrefix(path, "/api") || strings.HasPrefix(path, "/custom") || strings.HasPrefix(path, "/dav") || path == "/manifest.json" {
			c.Next()
			return
		}

		// 不存在的路徑和index.html均返回index.html
		if (path == "/index.html") || (path == "/") || !bootstrap.StaticFS.Exists("/", path) {
			// 讀取、取代站點設定
			options := model.GetSettingByNames("siteName", "siteKeywords", "siteScript",
				"pwa_small_icon")
			finalHTML := util.Replace(map[string]string{
				"{siteName}":       options["siteName"],
				"{siteDes}":        options["siteDes"],
				"{siteScript}":     options["siteScript"],
				"{pwa_small_icon}": options["pwa_small_icon"],
			}, fileContent)

			c.Header("Content-Type", "text/html")
			c.String(200, finalHTML)
			c.Abort()
			return
		}

		// 存在的靜態文件
		fileServer.ServeHTTP(c.Writer, c.Request)
		c.Abort()
	}
}
