package filesystem

import (
	"context"
	"io"
	"os"
	"path"

	model "github.com/cloudreve/Cloudreve/v3/models"
	"github.com/cloudreve/Cloudreve/v3/pkg/cache"
	"github.com/cloudreve/Cloudreve/v3/pkg/filesystem/driver/local"
	"github.com/cloudreve/Cloudreve/v3/pkg/filesystem/fsctx"
	"github.com/cloudreve/Cloudreve/v3/pkg/request"
	"github.com/cloudreve/Cloudreve/v3/pkg/serializer"
	"github.com/cloudreve/Cloudreve/v3/pkg/util"
	"github.com/gin-gonic/gin"
)

/* ================
	 上傳處理相關
   ================
*/

// Upload 上傳文件
func (fs *FileSystem) Upload(ctx context.Context, file FileHeader) (err error) {
	ctx = context.WithValue(ctx, fsctx.FileHeaderCtx, file)

	// 上傳前的鉤子
	err = fs.Trigger(ctx, "BeforeUpload")
	if err != nil {
		request.BlackHole(file)
		return err
	}

	// 生成檔案名和路徑,
	var savePath string
	// 如果是更新操作就從上下文中獲取
	if originFile, ok := ctx.Value(fsctx.FileModelCtx).(model.File); ok {
		savePath = originFile.SourceName
	} else {
		savePath = fs.GenerateSavePath(ctx, file)
	}
	ctx = context.WithValue(ctx, fsctx.SavePathCtx, savePath)

	// 處理用戶端未完成上傳時，關閉連接
	go fs.CancelUpload(ctx, savePath, file)

	// 儲存文件
	err = fs.Handler.Put(ctx, file, savePath, file.GetSize())
	if err != nil {
		fs.Trigger(ctx, "AfterUploadFailed")
		return err
	}

	// 上傳完成後的鉤子
	err = fs.Trigger(ctx, "AfterUpload")

	if err != nil {
		// 上傳完成後續處理失敗
		followUpErr := fs.Trigger(ctx, "AfterValidateFailed")
		// 失敗後再失敗...
		if followUpErr != nil {
			util.Log().Debug("AfterValidateFailed 鉤子執行失敗，%s", followUpErr)
		}

		return err
	}

	util.Log().Info(
		"新文件PUT:%s , 大小:%d, 上傳者:%s",
		file.GetFileName(),
		file.GetSize(),
		fs.User.Nick,
	)

	return nil
}

// GenerateSavePath 生成要存放文件的路徑
// TODO 完善測試
func (fs *FileSystem) GenerateSavePath(ctx context.Context, file FileHeader) string {
	if fs.User.Model.ID != 0 {
		return path.Join(
			fs.User.Policy.GeneratePath(
				fs.User.Model.ID,
				file.GetVirtualPath(),
			),
			fs.User.Policy.GenerateFileName(
				fs.User.Model.ID,
				file.GetFileName(),
			),
		)
	}

	// 匿名文件系統嘗試根據上下文中的上傳策略生成路徑
	var anonymousPolicy model.Policy
	if policy, ok := ctx.Value(fsctx.UploadPolicyCtx).(serializer.UploadPolicy); ok {
		anonymousPolicy = model.Policy{
			Type:         "remote",
			AutoRename:   policy.AutoRename,
			DirNameRule:  policy.SavePath,
			FileNameRule: policy.FileName,
		}
	}
	return path.Join(
		anonymousPolicy.GeneratePath(
			0,
			"",
		),
		anonymousPolicy.GenerateFileName(
			0,
			file.GetFileName(),
		),
	)
}

// CancelUpload 監測用戶端取消上傳
func (fs *FileSystem) CancelUpload(ctx context.Context, path string, file FileHeader) {
	var reqContext context.Context
	if ginCtx, ok := ctx.Value(fsctx.GinCtx).(*gin.Context); ok {
		reqContext = ginCtx.Request.Context()
	} else if reqCtx, ok := ctx.Value(fsctx.HTTPCtx).(context.Context); ok {
		reqContext = reqCtx
	} else {
		return
	}

	select {
	case <-reqContext.Done():
		select {
		case <-ctx.Done():
			// 客戶端正常關閉，不執行操作
		default:
			// 用戶端取消上傳，刪除暫存檔
			util.Log().Debug("用戶端取消上傳")
			if fs.Hooks["AfterUploadCanceled"] == nil {
				return
			}
			ctx = context.WithValue(ctx, fsctx.SavePathCtx, path)
			err := fs.Trigger(ctx, "AfterUploadCanceled")
			if err != nil {
				util.Log().Debug("執行 AfterUploadCanceled 鉤子出錯，%s", err)
			}
		}

	}
}

// GetUploadToken 生成新的上傳憑證
func (fs *FileSystem) GetUploadToken(ctx context.Context, path string, size uint64, name string) (*serializer.UploadCredential, error) {
	// 獲取相關有效期設定
	credentialTTL := model.GetIntSetting("upload_credential_timeout", 3600)
	callBackSessionTTL := model.GetIntSetting("upload_session_timeout", 86400)

	var err error

	// 檢查檔案大小
	if fs.User.Policy.MaxSize != 0 {
		if size > fs.User.Policy.MaxSize {
			return nil, ErrFileSizeTooBig
		}
	}

	// 是否需要預先生成儲存路徑
	var savePath string
	if fs.User.Policy.IsPathGenerateNeeded() {
		savePath = fs.GenerateSavePath(ctx, local.FileStream{Name: name, VirtualPath: path})
		ctx = context.WithValue(ctx, fsctx.SavePathCtx, savePath)
	}
	ctx = context.WithValue(ctx, fsctx.FileSizeCtx, size)

	// 獲取上傳憑證
	callbackKey := util.RandStringRunes(32)
	credential, err := fs.Handler.Token(ctx, int64(credentialTTL), callbackKey)
	if err != nil {
		return nil, serializer.NewError(serializer.CodeEncryptError, "無法獲取上傳憑證", err)
	}

	// 建立回調工作階段
	err = cache.Set(
		"callback_"+callbackKey,
		serializer.UploadSession{
			Key:         callbackKey,
			UID:         fs.User.ID,
			PolicyID:    fs.User.GetPolicyID(0),
			VirtualPath: path,
			Name:        name,
			Size:        size,
			SavePath:    savePath,
		},
		callBackSessionTTL,
	)
	if err != nil {
		return nil, err
	}

	return &credential, nil
}

// UploadFromStream 從文件流上傳文件
func (fs *FileSystem) UploadFromStream(ctx context.Context, src io.ReadCloser, dst string, size uint64) error {
	// 構建文件頭
	fileName := path.Base(dst)
	filePath := path.Dir(dst)
	fileData := local.FileStream{
		File:        src,
		Size:        size,
		Name:        fileName,
		VirtualPath: filePath,
	}

	// 給文件系統分配鉤子
	fs.Lock.Lock()
	if fs.Hooks == nil {
		fs.Use("BeforeUpload", HookValidateFile)
		fs.Use("BeforeUpload", HookValidateCapacity)
		fs.Use("AfterUploadCanceled", HookDeleteTempFile)
		fs.Use("AfterUploadCanceled", HookGiveBackCapacity)
		fs.Use("AfterUpload", GenericAfterUpload)
		fs.Use("AfterValidateFailed", HookDeleteTempFile)
		fs.Use("AfterValidateFailed", HookGiveBackCapacity)
		fs.Use("AfterUploadFailed", HookGiveBackCapacity)
	}
	fs.Lock.Unlock()

	// 開始上傳
	return fs.Upload(ctx, fileData)
}

// UploadFromPath 將本機已有文件上傳到使用者的文件系統
func (fs *FileSystem) UploadFromPath(ctx context.Context, src, dst string) error {
	// 重設儲存策略
	fs.Policy = &fs.User.Policy
	err := fs.DispatchHandler()
	if err != nil {
		return err
	}

	file, err := os.Open(util.RelativePath(src))
	if err != nil {
		return err
	}
	defer file.Close()

	// 獲取來源檔案大小
	fi, err := file.Stat()
	if err != nil {
		return err
	}
	size := fi.Size()

	// 開始上傳
	return fs.UploadFromStream(ctx, file, dst, uint64(size))
}
