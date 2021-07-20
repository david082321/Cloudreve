package aria2

import (
	model "github.com/cloudreve/Cloudreve/v3/models"
	"github.com/cloudreve/Cloudreve/v3/pkg/aria2"
	"github.com/cloudreve/Cloudreve/v3/pkg/filesystem"
	"github.com/cloudreve/Cloudreve/v3/pkg/serializer"
	"github.com/gin-gonic/gin"
)

// AddURLService 添加URL離線下載服務
type AddURLService struct {
	URL string `json:"url" binding:"required"`
	Dst string `json:"dst" binding:"required,min=1"`
}

// Add 建立新的連結離線下載任務
func (service *AddURLService) Add(c *gin.Context, taskType int) serializer.Response {
	// 建立文件系統
	fs, err := filesystem.NewFileSystemFromContext(c)
	if err != nil {
		return serializer.Err(serializer.CodePolicyNotAllowed, err.Error(), err)
	}
	defer fs.Recycle()

	// 檢查使用者群組權限
	if !fs.User.Group.OptionsSerialized.Aria2 {
		return serializer.Err(serializer.CodeGroupNotAllowed, "目前使用者群組無法進行此操作", nil)
	}

	// 存放目錄是否存在
	if exist, _ := fs.IsPathExist(service.Dst); !exist {
		return serializer.Err(serializer.CodeNotFound, "存放路徑不存在", nil)
	}

	// 建立任務
	task := &model.Download{
		Status: aria2.Ready,
		Type:   taskType,
		Dst:    service.Dst,
		UserID: fs.User.ID,
		Source: service.URL,
	}

	aria2.Lock.RLock()
	if err := aria2.Instance.CreateTask(task, fs.User.Group.OptionsSerialized.Aria2Options); err != nil {
		aria2.Lock.RUnlock()
		return serializer.Err(serializer.CodeNotSet, "任務建立失敗", err)
	}
	aria2.Lock.RUnlock()

	return serializer.Response{}
}
