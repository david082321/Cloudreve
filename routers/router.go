package routers

import (
	"github.com/cloudreve/Cloudreve/v3/middleware"
	"github.com/cloudreve/Cloudreve/v3/pkg/conf"
	"github.com/cloudreve/Cloudreve/v3/pkg/hashid"
	"github.com/cloudreve/Cloudreve/v3/pkg/util"
	"github.com/cloudreve/Cloudreve/v3/routers/controllers"
	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
)

// InitRouter 初始化路由
func InitRouter() *gin.Engine {
	if conf.SystemConfig.Mode == "master" {
		util.Log().Info("目前執行模式：Master")
		return InitMasterRouter()
	}
	util.Log().Info("目前執行模式：Slave")
	return InitSlaveRouter()

}

// InitSlaveRouter 初始化從機模式路由
func InitSlaveRouter() *gin.Engine {
	r := gin.Default()
	// 跨域相關
	InitCORS(r)
	v3 := r.Group("/api/v3/slave")
	// 鑒權中間件
	v3.Use(middleware.SignRequired())

	/*
		路由
	*/
	{
		// Ping
		v3.POST("ping", controllers.SlavePing)
		// 上傳
		v3.POST("upload", controllers.SlaveUpload)
		// 下載
		v3.GET("download/:speed/:path/:name", controllers.SlaveDownload)
		// 預覽 / 外鏈
		v3.GET("source/:speed/:path/:name", controllers.SlavePreview)
		// 縮圖
		v3.GET("thumb/:path", controllers.SlaveThumb)
		// 刪除文件
		v3.POST("delete", controllers.SlaveDelete)
		// 列出文件
		v3.POST("list", controllers.SlaveList)
	}
	return r
}

// InitCORS 初始化跨域配置
func InitCORS(router *gin.Engine) {
	if conf.CORSConfig.AllowOrigins[0] != "UNSET" {
		router.Use(cors.New(cors.Config{
			AllowOrigins:     conf.CORSConfig.AllowOrigins,
			AllowMethods:     conf.CORSConfig.AllowMethods,
			AllowHeaders:     conf.CORSConfig.AllowHeaders,
			AllowCredentials: conf.CORSConfig.AllowCredentials,
			ExposeHeaders:    conf.CORSConfig.ExposeHeaders,
		}))
		return
	}

	// slave模式下未啟動跨域的警告
	if conf.SystemConfig.Mode == "slave" {
		util.Log().Warning("目前作為儲存端（Slave）執行，但未啟用跨域配置，可能會導致 Master 端無法正常上傳文件")
	}
}

// InitMasterRouter 初始化主機模式路由
func InitMasterRouter() *gin.Engine {
	r := gin.Default()

	/*
		靜態資源
	*/
	r.Use(gzip.Gzip(gzip.DefaultCompression, gzip.WithExcludedPaths([]string{"/api/"})))
	r.Use(middleware.FrontendFileHandler())
	r.GET("manifest.json", controllers.Manifest)

	v3 := r.Group("/api/v3")

	/*
		中間件
	*/
	v3.Use(middleware.Session(conf.SystemConfig.SessionSecret))
	// 跨域相關
	InitCORS(r)
	// 測試模式加入Mock助手中間件
	if gin.Mode() == gin.TestMode {
		v3.Use(middleware.MockHelper())
	}
	v3.Use(middleware.CurrentUser())

	/*
		路由
	*/
	{
		// 全域設定相關
		site := v3.Group("site")
		{
			// 測試用路由
			site.GET("ping", controllers.Ping)
			// 驗證碼
			site.GET("captcha", controllers.Captcha)
			// 站點全域配置
			site.GET("config", middleware.CSRFInit(), controllers.SiteConfig)
		}

		// 使用者相關路由
		user := v3.Group("user")
		{
			// 使用者登入
			user.POST("session", middleware.CaptchaRequired("login_captcha"), controllers.UserLogin)
			// 使用者註冊
			user.POST("",
				middleware.IsFunctionEnabled("register_enabled"),
				middleware.CaptchaRequired("reg_captcha"),
				controllers.UserRegister,
			)
			// 用二步驗證戶登入
			user.POST("2fa", controllers.User2FALogin)
			// 發送密碼重設郵件
			user.POST("reset", middleware.CaptchaRequired("forget_captcha"), controllers.UserSendReset)
			// 透過郵件裡的連結重設密碼
			user.PATCH("reset", controllers.UserReset)
			// 郵件啟動
			user.GET("activate/:id",
				middleware.SignRequired(),
				middleware.HashID(hashid.UserID),
				controllers.UserActivate,
			)
			// WebAuthn登入初始化
			user.GET("authn/:username",
				middleware.IsFunctionEnabled("authn_enabled"),
				controllers.StartLoginAuthn,
			)
			// WebAuthn登入
			user.POST("authn/finish/:username",
				middleware.IsFunctionEnabled("authn_enabled"),
				controllers.FinishLoginAuthn,
			)
			// 獲取使用者首頁展示用分享
			user.GET("profile/:id",
				middleware.HashID(hashid.UserID),
				controllers.GetUserShare,
			)
			// 獲取使用者大頭貼
			user.GET("avatar/:id/:size",
				middleware.HashID(hashid.UserID),
				controllers.GetUserAvatar,
			)
		}

		// 需要攜帶簽名驗證的
		sign := v3.Group("")
		sign.Use(middleware.SignRequired())
		{
			file := sign.Group("file")
			{
				// 文件外鏈（直接輸出文件資料）
				file.GET("get/:id/:name", controllers.AnonymousGetContent)
				// 文件外鏈(301跳轉)
				file.GET("source/:id/:name", controllers.AnonymousPermLink)
				// 下載已經打包好的文件
				file.GET("archive/:id/archive.zip", controllers.DownloadArchive)
				// 下載文件
				file.GET("download/:id", controllers.Download)
			}
		}

		// 回調介面
		callback := v3.Group("callback")
		{
			// 遠端策略上傳回調
			callback.POST(
				"remote/:key",
				middleware.RemoteCallbackAuth(),
				controllers.RemoteCallback,
			)
			// 七牛策略上傳回調
			callback.POST(
				"qiniu/:key",
				middleware.QiniuCallbackAuth(),
				controllers.QiniuCallback,
			)
			// 阿里雲OSS策略上傳回調
			callback.POST(
				"oss/:key",
				middleware.OSSCallbackAuth(),
				controllers.OSSCallback,
			)
			// 又拍雲策略上傳回調
			callback.POST(
				"upyun/:key",
				middleware.UpyunCallbackAuth(),
				controllers.UpyunCallback,
			)
			onedrive := callback.Group("onedrive")
			{
				// 文件上傳完成
				onedrive.POST(
					"finish/:key",
					middleware.OneDriveCallbackAuth(),
					controllers.OneDriveCallback,
				)
				// 文件上傳完成
				onedrive.GET(
					"auth",
					controllers.OneDriveOAuth,
				)
			}
			// 騰訊雲COS策略上傳回調
			callback.GET(
				"cos/:key",
				middleware.COSCallbackAuth(),
				controllers.COSCallback,
			)
			// AWS S3策略上傳回調
			callback.GET(
				"s3/:key",
				middleware.S3CallbackAuth(),
				controllers.S3Callback,
			)
		}

		// 分享相關
		share := v3.Group("share", middleware.ShareAvailable())
		{
			// 獲取分享
			share.GET("info/:id", controllers.GetShare)
			// 建立文件下載工作階段
			share.PUT("download/:id",
				middleware.CheckShareUnlocked(),
				middleware.BeforeShareDownload(),
				controllers.GetShareDownload,
			)
			// 預覽分享文件
			share.GET("preview/:id",
				middleware.CSRFCheck(),
				middleware.CheckShareUnlocked(),
				middleware.ShareCanPreview(),
				middleware.BeforeShareDownload(),
				controllers.PreviewShare,
			)
			// 取得Office文件預覽地址
			share.GET("doc/:id",
				middleware.CheckShareUnlocked(),
				middleware.ShareCanPreview(),
				middleware.BeforeShareDownload(),
				controllers.GetShareDocPreview,
			)
			// 獲取文字文件內容
			share.GET("content/:id",
				middleware.CheckShareUnlocked(),
				middleware.BeforeShareDownload(),
				controllers.PreviewShareText,
			)
			// 分享目錄列文件
			share.GET("list/:id/*path",
				middleware.CheckShareUnlocked(),
				controllers.ListSharedFolder,
			)
			// 歸檔打包下載
			share.POST("archive/:id",
				middleware.CheckShareUnlocked(),
				middleware.BeforeShareDownload(),
				controllers.ArchiveShare,
			)
			// 獲取README文字文件內容
			share.GET("readme/:id",
				middleware.CheckShareUnlocked(),
				controllers.PreviewShareReadme,
			)
			// 獲取縮圖
			share.GET("thumb/:id/:file",
				middleware.CheckShareUnlocked(),
				middleware.ShareCanPreview(),
				controllers.ShareThumb,
			)
			// 搜尋公共分享
			v3.Group("share").GET("search", controllers.SearchShare)
		}

		// 需要登入保護的
		auth := v3.Group("")
		auth.Use(middleware.AuthRequired())
		{
			// 管理
			admin := auth.Group("admin", middleware.IsAdmin())
			{
				// 獲取站點概況
				admin.GET("summary", controllers.AdminSummary)
				// 獲取社群新聞
				admin.GET("news", controllers.AdminNews)
				// 更改設定
				admin.PATCH("setting", controllers.AdminChangeSetting)
				// 獲取設定
				admin.POST("setting", controllers.AdminGetSetting)
				// 獲取使用者群組列表
				admin.GET("groups", controllers.AdminGetGroups)
				// 重新載入子服務
				admin.GET("reload/:service", controllers.AdminReloadService)
				// 重新載入子服務
				admin.POST("mailTest", controllers.AdminSendTestMail)

				// 離線下載相關
				aria2 := admin.Group("aria2")
				{
					// 測試連接配置
					aria2.POST("test", controllers.AdminTestAria2)
				}

				// 儲存策略管理
				policy := admin.Group("policy")
				{
					// 列出儲存策略
					policy.POST("list", controllers.AdminListPolicy)
					// 測試本機路徑可用性
					policy.POST("test/path", controllers.AdminTestPath)
					// 測試從機通信
					policy.POST("test/slave", controllers.AdminTestSlave)
					// 建立儲存策略
					policy.POST("", controllers.AdminAddPolicy)
					// 建立跨域策略
					policy.POST("cors", controllers.AdminAddCORS)
					// 建立COS回調函數
					policy.POST("scf", controllers.AdminAddSCF)
					// 獲取 OneDrive OAuth URL
					policy.GET(":id/oauth", controllers.AdminOneDriveOAuth)
					// 獲取 儲存策略
					policy.GET(":id", controllers.AdminGetPolicy)
					// 刪除 儲存策略
					policy.DELETE(":id", controllers.AdminDeletePolicy)
				}

				// 使用者群組管理
				group := admin.Group("group")
				{
					// 列出使用者群組
					group.POST("list", controllers.AdminListGroup)
					// 獲取使用者群組
					group.GET(":id", controllers.AdminGetGroup)
					// 建立/儲存使用者群組
					group.POST("", controllers.AdminAddGroup)
					// 刪除
					group.DELETE(":id", controllers.AdminDeleteGroup)
				}

				user := admin.Group("user")
				{
					// 列出使用者
					user.POST("list", controllers.AdminListUser)
					// 獲取使用者
					user.GET(":id", controllers.AdminGetUser)
					// 建立/儲存使用者
					user.POST("", controllers.AdminAddUser)
					// 刪除
					user.POST("delete", controllers.AdminDeleteUser)
					// 封禁/解封使用者
					user.PATCH("ban/:id", controllers.AdminBanUser)
				}

				file := admin.Group("file")
				{
					// 列出文件
					file.POST("list", controllers.AdminListFile)
					// 預覽文件
					file.GET("preview/:id", controllers.AdminGetFile)
					// 刪除
					file.POST("delete", controllers.AdminDeleteFile)
					// 列出使用者或外部文件系統目錄
					file.GET("folders/:type/:id/*path",
						controllers.AdminListFolders)
				}

				share := admin.Group("share")
				{
					// 列出分享
					share.POST("list", controllers.AdminListShare)
					// 刪除
					share.POST("delete", controllers.AdminDeleteShare)
				}

				download := admin.Group("download")
				{
					// 列出任務
					download.POST("list", controllers.AdminListDownload)
					// 刪除
					download.POST("delete", controllers.AdminDeleteDownload)
				}

				task := admin.Group("task")
				{
					// 列出任務
					task.POST("list", controllers.AdminListTask)
					// 刪除
					task.POST("delete", controllers.AdminDeleteTask)
					// 建立文件匯入任務
					task.POST("import", controllers.AdminCreateImportTask)
				}

			}

			// 使用者
			user := auth.Group("user")
			{
				// 目前登入使用者訊息
				user.GET("me", controllers.UserMe)
				// 儲存訊息
				user.GET("storage", controllers.UserStorage)
				// 退出登入
				user.DELETE("session", controllers.UserSignOut)

				// WebAuthn 註冊相關
				authn := user.Group("authn",
					middleware.IsFunctionEnabled("authn_enabled"))
				{
					authn.PUT("", controllers.StartRegAuthn)
					authn.PUT("finish", controllers.FinishRegAuthn)
				}

				// 使用者設定
				setting := user.Group("setting")
				{
					// 任務佇列
					setting.GET("tasks", controllers.UserTasks)
					// 獲取目前使用者設定
					setting.GET("", controllers.UserSetting)
					// 從文件上傳大頭貼
					setting.POST("avatar", controllers.UploadAvatar)
					// 設定為Gravatar大頭貼
					setting.PUT("avatar", controllers.UseGravatar)
					// 更改使用者設定
					setting.PATCH(":option", controllers.UpdateOption)
					// 獲得二步驗證初始化訊息
					setting.GET("2fa", controllers.UserInit2FA)
				}
			}

			// 文件
			file := auth.Group("file", middleware.HashID(hashid.FileID))
			{
				// 文件上傳
				file.POST("upload", controllers.FileUploadStream)
				// 獲取上傳憑證
				file.GET("upload/credential", controllers.GetUploadCredential)
				// 更新文件
				file.PUT("update/:id", controllers.PutContent)
				// 建立空白文件
				file.POST("create", controllers.CreateFile)
				// 建立文件下載工作階段
				file.PUT("download/:id", controllers.CreateDownloadSession)
				// 預覽文件
				file.GET("preview/:id", controllers.Preview)
				// 獲取文字文件內容
				file.GET("content/:id", controllers.PreviewText)
				// 取得Office文件預覽地址
				file.GET("doc/:id", controllers.GetDocPreview)
				// 獲取縮圖
				file.GET("thumb/:id", controllers.Thumb)
				// 取得文件外鏈
				file.GET("source/:id", controllers.GetSource)
				// 打包要下載的文件
				file.POST("archive", controllers.Archive)
				// 建立文件壓縮任務
				file.POST("compress", controllers.Compress)
				// 建立文件解壓縮任務
				file.POST("decompress", controllers.Decompress)
				// 建立文件解壓縮任務
				file.GET("search/:type/:keywords", controllers.SearchFile)
			}

			// 離線下載任務
			aria2 := auth.Group("aria2")
			{
				// 建立URL下載任務
				aria2.POST("url", controllers.AddAria2URL)
				// 建立種子下載任務
				aria2.POST("torrent/:id", middleware.HashID(hashid.FileID), controllers.AddAria2Torrent)
				// 重新選擇要下載的文件
				aria2.PUT("select/:gid", controllers.SelectAria2File)
				// 取消或刪除下載任務
				aria2.DELETE("task/:gid", controllers.CancelAria2Download)
				// 獲取正在下載中的任務
				aria2.GET("downloading", controllers.ListDownloading)
				// 獲取已完成的任務
				aria2.GET("finished", controllers.ListFinished)
			}

			// 目錄
			directory := auth.Group("directory")
			{
				// 建立目錄
				directory.PUT("", controllers.CreateDirectory)
				// 列出目錄下內容
				directory.GET("*path", controllers.ListDirectory)
			}

			// 物件，文件和目錄的抽象
			object := auth.Group("object")
			{
				// 刪除物件
				object.DELETE("", controllers.Delete)
				// 移動物件
				object.PATCH("", controllers.Move)
				// 複製物件
				object.POST("copy", controllers.Copy)
				// 重新命名物件
				object.POST("rename", controllers.Rename)
				// 獲取物件屬性
				object.GET("property/:id", controllers.GetProperty)
			}

			// 分享
			share := auth.Group("share")
			{
				// 建立新分享
				share.POST("", controllers.CreateShare)
				// 列出我的分享
				share.GET("", controllers.ListShare)
				// 更新分享屬性
				share.PATCH(":id",
					middleware.ShareAvailable(),
					middleware.ShareOwner(),
					controllers.UpdateShare,
				)
				// 刪除分享
				share.DELETE(":id",
					controllers.DeleteShare,
				)
			}

			// 使用者標籤
			tag := auth.Group("tag")
			{
				// 建立文件分類標籤
				tag.POST("filter", controllers.CreateFilterTag)
				// 建立目錄捷徑標籤
				tag.POST("link", controllers.CreateLinkTag)
				// 刪除標籤
				tag.DELETE(":id", middleware.HashID(hashid.TagID), controllers.DeleteTag)
			}

			// WebDAV管理相關
			webdav := auth.Group("webdav")
			{
				// 獲取帳號訊息
				webdav.GET("accounts", controllers.GetWebDAVAccounts)
				// 建立帳號
				webdav.POST("accounts", controllers.CreateWebDAVAccounts)
				// 刪除帳號
				webdav.DELETE("accounts/:id", controllers.DeleteWebDAVAccounts)
			}

		}

	}

	// 初始化WebDAV相關路由
	initWebDAV(r.Group("dav"))
	return r
}

// initWebDAV 初始化WebDAV相關路由
func initWebDAV(group *gin.RouterGroup) {
	{
		group.Use(middleware.WebDAVAuth())

		group.Any("/*path", controllers.ServeWebDAV)
		group.Any("", controllers.ServeWebDAV)
		group.Handle("PROPFIND", "/*path", controllers.ServeWebDAV)
		group.Handle("PROPFIND", "", controllers.ServeWebDAV)
		group.Handle("MKCOL", "/*path", controllers.ServeWebDAV)
		group.Handle("LOCK", "/*path", controllers.ServeWebDAV)
		group.Handle("UNLOCK", "/*path", controllers.ServeWebDAV)
		group.Handle("PROPPATCH", "/*path", controllers.ServeWebDAV)
		group.Handle("COPY", "/*path", controllers.ServeWebDAV)
		group.Handle("MOVE", "/*path", controllers.ServeWebDAV)

	}
}
