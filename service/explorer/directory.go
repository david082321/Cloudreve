package explorer

import (
	"context"

	"github.com/cloudreve/Cloudreve/v3/pkg/filesystem"
	"github.com/cloudreve/Cloudreve/v3/pkg/hashid"
	"github.com/cloudreve/Cloudreve/v3/pkg/serializer"
	"github.com/gin-gonic/gin"
)

// DirectoryService 建立新目錄服務
type DirectoryService struct {
	Path string `uri:"path" json:"path" binding:"required,min=1,max=65535"`
}

// ListDirectory 列出目錄內容
func (service *DirectoryService) ListDirectory(c *gin.Context) serializer.Response {
	// 建立文件系統
	fs, err := filesystem.NewFileSystemFromContext(c)
	if err != nil {
		return serializer.Err(serializer.CodePolicyNotAllowed, err.Error(), err)
	}
	defer fs.Recycle()

	// 上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 獲取子項目
	objects, err := fs.List(ctx, service.Path, nil)
	if err != nil {
		return serializer.Err(serializer.CodeNotSet, err.Error(), err)
	}

	var parentID uint
	if len(fs.DirTarget) > 0 {
		parentID = fs.DirTarget[0].ID
	}

	return serializer.Response{
		Code: 0,
		Data: map[string]interface{}{
			"parent":  hashid.HashID(parentID, hashid.FolderID),
			"objects": objects,
		},
	}
}

// CreateDirectory 建立目錄
func (service *DirectoryService) CreateDirectory(c *gin.Context) serializer.Response {
	// 建立文件系統
	fs, err := filesystem.NewFileSystemFromContext(c)
	if err != nil {
		return serializer.Err(serializer.CodePolicyNotAllowed, err.Error(), err)
	}
	defer fs.Recycle()

	// 上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 建立目錄
	_, err = fs.CreateDirectory(ctx, service.Path)
	if err != nil {
		return serializer.Err(serializer.CodeCreateFolderFailed, err.Error(), err)
	}
	return serializer.Response{
		Code: 0,
	}

}
