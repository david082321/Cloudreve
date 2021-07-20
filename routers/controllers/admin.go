package controllers

import (
	"io"

	"github.com/cloudreve/Cloudreve/v3/pkg/aria2"
	"github.com/cloudreve/Cloudreve/v3/pkg/email"
	"github.com/cloudreve/Cloudreve/v3/pkg/request"
	"github.com/cloudreve/Cloudreve/v3/pkg/serializer"
	"github.com/cloudreve/Cloudreve/v3/service/admin"
	"github.com/gin-gonic/gin"
)

// AdminSummary 獲取管理站點概況
func AdminSummary(c *gin.Context) {
	var service admin.NoParamService
	if err := c.ShouldBindUri(&service); err == nil {
		res := service.Summary()
		c.JSON(200, res)
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}

// AdminNews 獲取社群新聞
func AdminNews(c *gin.Context) {
	r := request.HTTPClient{}
	res := r.Request("GET", "https://forum.cloudreve.org/api/discussions?include=startUser,lastUser,startPost,tags&filter[q]= tag:notice&sort=-startTime&page[limit]=10", nil)
	if res.Err == nil {
		io.Copy(c.Writer, res.Response.Body)
	}
}

// AdminChangeSetting 獲取站點設定項
func AdminChangeSetting(c *gin.Context) {
	var service admin.BatchSettingChangeService
	if err := c.ShouldBindJSON(&service); err == nil {
		res := service.Change()
		c.JSON(200, res)
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}

// AdminGetSetting 獲取站點設定
func AdminGetSetting(c *gin.Context) {
	var service admin.BatchSettingGet
	if err := c.ShouldBindJSON(&service); err == nil {
		res := service.Get()
		c.JSON(200, res)
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}

// AdminGetGroups 獲取使用者群組列表
func AdminGetGroups(c *gin.Context) {
	var service admin.NoParamService
	if err := c.ShouldBindUri(&service); err == nil {
		res := service.GroupList()
		c.JSON(200, res)
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}

// AdminReloadService 重新載入子服務
func AdminReloadService(c *gin.Context) {
	service := c.Param("service")
	switch service {
	case "email":
		email.Init()
	case "aria2":
		aria2.Init(true)
	}

	c.JSON(200, serializer.Response{})
}

// AdminSendTestMail 發送測試郵件
func AdminSendTestMail(c *gin.Context) {
	var service admin.MailTestService
	if err := c.ShouldBindJSON(&service); err == nil {
		res := service.Send()
		c.JSON(200, res)
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}

// AdminTestAria2 測試aria2連接
func AdminTestAria2(c *gin.Context) {
	var service admin.Aria2TestService
	if err := c.ShouldBindJSON(&service); err == nil {
		res := service.Test()
		c.JSON(200, res)
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}

// AdminListPolicy 列出儲存策略
func AdminListPolicy(c *gin.Context) {
	var service admin.AdminListService
	if err := c.ShouldBindJSON(&service); err == nil {
		res := service.Policies()
		c.JSON(200, res)
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}

// AdminTestPath 測試本機路徑可用性
func AdminTestPath(c *gin.Context) {
	var service admin.PathTestService
	if err := c.ShouldBindJSON(&service); err == nil {
		res := service.Test()
		c.JSON(200, res)
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}

// AdminTestSlave 測試從機可用性
func AdminTestSlave(c *gin.Context) {
	var service admin.SlaveTestService
	if err := c.ShouldBindJSON(&service); err == nil {
		res := service.Test()
		c.JSON(200, res)
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}

// AdminAddPolicy 建立儲存策略
func AdminAddPolicy(c *gin.Context) {
	var service admin.AddPolicyService
	if err := c.ShouldBindJSON(&service); err == nil {
		res := service.Add()
		c.JSON(200, res)
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}

// AdminAddCORS 建立跨域策略
func AdminAddCORS(c *gin.Context) {
	var service admin.PolicyService
	if err := c.ShouldBindJSON(&service); err == nil {
		res := service.AddCORS()
		c.JSON(200, res)
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}

// AdminAddSCF 建立回調函數
func AdminAddSCF(c *gin.Context) {
	var service admin.PolicyService
	if err := c.ShouldBindJSON(&service); err == nil {
		res := service.AddSCF()
		c.JSON(200, res)
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}

// AdminOneDriveOAuth 獲取 OneDrive OAuth URL
func AdminOneDriveOAuth(c *gin.Context) {
	var service admin.PolicyService
	if err := c.ShouldBindUri(&service); err == nil {
		res := service.GetOAuth(c)
		c.JSON(200, res)
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}

// AdminGetPolicy 獲取儲存策略詳情
func AdminGetPolicy(c *gin.Context) {
	var service admin.PolicyService
	if err := c.ShouldBindUri(&service); err == nil {
		res := service.Get()
		c.JSON(200, res)
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}

// AdminDeletePolicy 刪除儲存策略
func AdminDeletePolicy(c *gin.Context) {
	var service admin.PolicyService
	if err := c.ShouldBindUri(&service); err == nil {
		res := service.Delete()
		c.JSON(200, res)
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}

// AdminListGroup 列出使用者群組
func AdminListGroup(c *gin.Context) {
	var service admin.AdminListService
	if err := c.ShouldBindJSON(&service); err == nil {
		res := service.Groups()
		c.JSON(200, res)
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}

// AdminAddGroup 建立使用者群組
func AdminAddGroup(c *gin.Context) {
	var service admin.AddGroupService
	if err := c.ShouldBindJSON(&service); err == nil {
		res := service.Add()
		c.JSON(200, res)
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}

// AdminDeleteGroup 刪除使用者群組
func AdminDeleteGroup(c *gin.Context) {
	var service admin.GroupService
	if err := c.ShouldBindUri(&service); err == nil {
		res := service.Delete()
		c.JSON(200, res)
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}

// AdminGetGroup 獲取使用者群組詳情
func AdminGetGroup(c *gin.Context) {
	var service admin.GroupService
	if err := c.ShouldBindUri(&service); err == nil {
		res := service.Get()
		c.JSON(200, res)
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}

// AdminListUser 列出使用者
func AdminListUser(c *gin.Context) {
	var service admin.AdminListService
	if err := c.ShouldBindJSON(&service); err == nil {
		res := service.Users()
		c.JSON(200, res)
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}

// AdminAddUser 建立使用者群組
func AdminAddUser(c *gin.Context) {
	var service admin.AddUserService
	if err := c.ShouldBindJSON(&service); err == nil {
		res := service.Add()
		c.JSON(200, res)
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}

// AdminGetUser 獲取使用者詳情
func AdminGetUser(c *gin.Context) {
	var service admin.UserService
	if err := c.ShouldBindUri(&service); err == nil {
		res := service.Get()
		c.JSON(200, res)
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}

// AdminDeleteUser 批次刪除使用者
func AdminDeleteUser(c *gin.Context) {
	var service admin.UserBatchService
	if err := c.ShouldBindJSON(&service); err == nil {
		res := service.Delete()
		c.JSON(200, res)
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}

// AdminBanUser 封禁/解封使用者
func AdminBanUser(c *gin.Context) {
	var service admin.UserService
	if err := c.ShouldBindUri(&service); err == nil {
		res := service.Ban()
		c.JSON(200, res)
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}

// AdminListFile 列出文件
func AdminListFile(c *gin.Context) {
	var service admin.AdminListService
	if err := c.ShouldBindJSON(&service); err == nil {
		res := service.Files()
		c.JSON(200, res)
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}

// AdminGetFile 獲取文件
func AdminGetFile(c *gin.Context) {
	var service admin.FileService
	if err := c.ShouldBindUri(&service); err == nil {
		res := service.Get(c)
		// 是否需要重定向
		if res.Code == -301 {
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

// AdminDeleteFile 批次刪除文件
func AdminDeleteFile(c *gin.Context) {
	var service admin.FileBatchService
	if err := c.ShouldBindJSON(&service); err == nil {
		res := service.Delete(c)
		c.JSON(200, res)
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}

// AdminListShare 列出分享
func AdminListShare(c *gin.Context) {
	var service admin.AdminListService
	if err := c.ShouldBindJSON(&service); err == nil {
		res := service.Shares()
		c.JSON(200, res)
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}

// AdminDeleteShare 批次刪除分享
func AdminDeleteShare(c *gin.Context) {
	var service admin.ShareBatchService
	if err := c.ShouldBindJSON(&service); err == nil {
		res := service.Delete(c)
		c.JSON(200, res)
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}

// AdminListDownload 列出離線下載任務
func AdminListDownload(c *gin.Context) {
	var service admin.AdminListService
	if err := c.ShouldBindJSON(&service); err == nil {
		res := service.Downloads()
		c.JSON(200, res)
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}

// AdminDeleteDownload 批次刪除任務
func AdminDeleteDownload(c *gin.Context) {
	var service admin.TaskBatchService
	if err := c.ShouldBindJSON(&service); err == nil {
		res := service.Delete(c)
		c.JSON(200, res)
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}

// AdminListTask 列出任務
func AdminListTask(c *gin.Context) {
	var service admin.AdminListService
	if err := c.ShouldBindJSON(&service); err == nil {
		res := service.Tasks()
		c.JSON(200, res)
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}

// AdminDeleteTask 批次刪除任務
func AdminDeleteTask(c *gin.Context) {
	var service admin.TaskBatchService
	if err := c.ShouldBindJSON(&service); err == nil {
		res := service.DeleteGeneral(c)
		c.JSON(200, res)
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}

// AdminCreateImportTask 建立文件匯入任務
func AdminCreateImportTask(c *gin.Context) {
	var service admin.ImportTaskService
	if err := c.ShouldBindJSON(&service); err == nil {
		res := service.Create(c, CurrentUser(c))
		c.JSON(200, res)
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}

// AdminListFolders 列出使用者或外部文件系統目錄
func AdminListFolders(c *gin.Context) {
	var service admin.ListFolderService
	if err := c.ShouldBindUri(&service); err == nil {
		res := service.List(c)
		c.JSON(200, res)
	} else {
		c.JSON(200, ErrorResponse(err))
	}
}
