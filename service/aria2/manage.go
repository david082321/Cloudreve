package aria2

import (
	model "github.com/cloudreve/Cloudreve/v3/models"
	"github.com/cloudreve/Cloudreve/v3/pkg/aria2"
	"github.com/cloudreve/Cloudreve/v3/pkg/serializer"
	"github.com/gin-gonic/gin"
)

// SelectFileService 選擇要下載的文件服務
type SelectFileService struct {
	Indexes []int `json:"indexes" binding:"required"`
}

// DownloadTaskService 下載任務管理服務
type DownloadTaskService struct {
	GID string `uri:"gid" binding:"required"`
}

// DownloadListService 下載列表服務
type DownloadListService struct {
	Page uint `form:"page"`
}

// Finished 獲取已完成的任務
func (service *DownloadListService) Finished(c *gin.Context, user *model.User) serializer.Response {
	// 尋找下載記錄
	downloads := model.GetDownloadsByStatusAndUser(service.Page, user.ID, aria2.Error, aria2.Complete, aria2.Canceled, aria2.Unknown)
	return serializer.BuildFinishedListResponse(downloads)
}

// Downloading 獲取正在下載中的任務
func (service *DownloadListService) Downloading(c *gin.Context, user *model.User) serializer.Response {
	// 尋找下載記錄
	downloads := model.GetDownloadsByStatusAndUser(service.Page, user.ID, aria2.Downloading, aria2.Paused, aria2.Ready)
	return serializer.BuildDownloadingResponse(downloads)
}

// Delete 取消或刪除下載任務
func (service *DownloadTaskService) Delete(c *gin.Context) serializer.Response {
	userCtx, _ := c.Get("user")
	user := userCtx.(*model.User)

	// 尋找下載記錄
	download, err := model.GetDownloadByGid(c.Param("gid"), user.ID)
	if err != nil {
		return serializer.Err(serializer.CodeNotFound, "下載記錄不存在", err)
	}

	if download.Status >= aria2.Error {
		// 如果任務已完成，則刪除任務記錄
		if err := download.Delete(); err != nil {
			return serializer.Err(serializer.CodeDBError, "任務記錄刪除失敗", err)
		}
		return serializer.Response{}
	}

	// 取消任務
	aria2.Lock.RLock()
	defer aria2.Lock.RUnlock()
	if err := aria2.Instance.Cancel(download); err != nil {
		return serializer.Err(serializer.CodeNotSet, "操作失敗", err)
	}

	return serializer.Response{}
}

// Select 選取要下載的文件
func (service *SelectFileService) Select(c *gin.Context) serializer.Response {
	userCtx, _ := c.Get("user")
	user := userCtx.(*model.User)

	// 尋找下載記錄
	download, err := model.GetDownloadByGid(c.Param("gid"), user.ID)
	if err != nil {
		return serializer.Err(serializer.CodeNotFound, "下載記錄不存在", err)
	}

	if download.StatusInfo.BitTorrent.Mode != "multi" || (download.Status != aria2.Downloading && download.Status != aria2.Paused) {
		return serializer.Err(serializer.CodeNoPermissionErr, "此下載任務無法選取文件", err)
	}

	// 選取下載
	aria2.Lock.RLock()
	defer aria2.Lock.RUnlock()
	if err := aria2.Instance.Select(download, service.Indexes); err != nil {
		return serializer.Err(serializer.CodeNotSet, "操作失敗", err)
	}

	return serializer.Response{}

}
