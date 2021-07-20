package explorer

import (
	"context"

	"github.com/cloudreve/Cloudreve/v3/pkg/filesystem"
	"github.com/cloudreve/Cloudreve/v3/pkg/filesystem/fsctx"
	"github.com/cloudreve/Cloudreve/v3/pkg/serializer"
	"github.com/gin-gonic/gin"
)

// UploadCredentialService 獲取上傳憑證服務
type UploadCredentialService struct {
	Path string `form:"path" binding:"required"`
	Size uint64 `form:"size" binding:"min=0"`
	Name string `form:"name"`
	Type string `form:"type"`
}

// Get 獲取新的上傳憑證
func (service *UploadCredentialService) Get(ctx context.Context, c *gin.Context) serializer.Response {
	// 建立文件系統
	fs, err := filesystem.NewFileSystemFromContext(c)
	if err != nil {
		return serializer.Err(serializer.CodePolicyNotAllowed, err.Error(), err)
	}

	// 儲存策略是否一致
	if service.Type != "" {
		if service.Type != fs.User.Policy.Type {
			return serializer.Err(serializer.CodePolicyNotAllowed, "儲存策略已變更，請重新整理頁面", nil)
		}
	}

	ctx = context.WithValue(ctx, fsctx.GinCtx, c)
	credential, err := fs.GetUploadToken(ctx, service.Path, service.Size, service.Name)
	if err != nil {
		return serializer.Err(serializer.CodeNotSet, err.Error(), err)
	}

	return serializer.Response{
		Code: 0,
		Data: credential,
	}
}
