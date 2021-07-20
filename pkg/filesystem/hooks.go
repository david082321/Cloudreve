package filesystem

import (
	"context"
	"errors"
	"io/ioutil"
	"strings"
	"sync"

	model "github.com/cloudreve/Cloudreve/v3/models"
	"github.com/cloudreve/Cloudreve/v3/pkg/conf"
	"github.com/cloudreve/Cloudreve/v3/pkg/filesystem/fsctx"
	"github.com/cloudreve/Cloudreve/v3/pkg/request"
	"github.com/cloudreve/Cloudreve/v3/pkg/serializer"
	"github.com/cloudreve/Cloudreve/v3/pkg/util"
)

// Hook 鉤子函數
type Hook func(ctx context.Context, fs *FileSystem) error

// Use 注入鉤子
func (fs *FileSystem) Use(name string, hook Hook) {
	if fs.Hooks == nil {
		fs.Hooks = make(map[string][]Hook)
	}
	if _, ok := fs.Hooks[name]; ok {
		fs.Hooks[name] = append(fs.Hooks[name], hook)
		return
	}
	fs.Hooks[name] = []Hook{hook}
}

// CleanHooks 清空鉤子,name為空表示全部清空
func (fs *FileSystem) CleanHooks(name string) {
	if name == "" {
		fs.Hooks = nil
	} else {
		delete(fs.Hooks, name)
	}
}

// Trigger 觸發鉤子,遇到第一個錯誤時
// 返回錯誤，後續鉤子不會繼續執行
func (fs *FileSystem) Trigger(ctx context.Context, name string) error {
	if hooks, ok := fs.Hooks[name]; ok {
		for _, hook := range hooks {
			err := hook(ctx, fs)
			if err != nil {
				util.Log().Warning("鉤子執行失敗：%s", err)
				return err
			}
		}
	}
	return nil
}

// HookIsFileExist 檢查虛擬路徑文件是否存在
func HookIsFileExist(ctx context.Context, fs *FileSystem) error {
	filePath := ctx.Value(fsctx.PathCtx).(string)
	if ok, _ := fs.IsFileExist(filePath); ok {
		return nil
	}
	return ErrObjectNotExist
}

// HookSlaveUploadValidate Slave模式下對文件上傳的一系列驗證
func HookSlaveUploadValidate(ctx context.Context, fs *FileSystem) error {
	file := ctx.Value(fsctx.FileHeaderCtx).(FileHeader)
	policy := ctx.Value(fsctx.UploadPolicyCtx).(serializer.UploadPolicy)

	// 驗證單文件尺寸
	if policy.MaxSize > 0 {
		if file.GetSize() > policy.MaxSize {
			return ErrFileSizeTooBig
		}
	}

	// 驗證檔案名
	if !fs.ValidateLegalName(ctx, file.GetFileName()) {
		return ErrIllegalObjectName
	}

	// 驗證副檔名
	if len(policy.AllowedExtension) > 0 && !IsInExtensionList(policy.AllowedExtension, file.GetFileName()) {
		return ErrFileExtensionNotAllowed
	}

	return nil
}

// HookValidateFile 一系列對文件檢驗的集合
func HookValidateFile(ctx context.Context, fs *FileSystem) error {
	file := ctx.Value(fsctx.FileHeaderCtx).(FileHeader)

	// 驗證單文件尺寸
	if !fs.ValidateFileSize(ctx, file.GetSize()) {
		return ErrFileSizeTooBig
	}

	// 驗證檔案名
	if !fs.ValidateLegalName(ctx, file.GetFileName()) {
		return ErrIllegalObjectName
	}

	// 驗證副檔名
	if !fs.ValidateExtension(ctx, file.GetFileName()) {
		return ErrFileExtensionNotAllowed
	}

	return nil

}

// HookResetPolicy 重設儲存策略為上下文已有文件
func HookResetPolicy(ctx context.Context, fs *FileSystem) error {
	originFile, ok := ctx.Value(fsctx.FileModelCtx).(model.File)
	if !ok {
		return ErrObjectNotExist
	}

	fs.Policy = originFile.GetPolicy()
	fs.User.Policy = *fs.Policy
	return fs.DispatchHandler()
}

// HookValidateCapacity 驗證並扣除使用者容量，包含資料庫操作
func HookValidateCapacity(ctx context.Context, fs *FileSystem) error {
	file := ctx.Value(fsctx.FileHeaderCtx).(FileHeader)
	// 驗證並扣除容量
	if !fs.ValidateCapacity(ctx, file.GetSize()) {
		return ErrInsufficientCapacity
	}
	return nil
}

// HookValidateCapacityWithoutIncrease 驗證使用者容量，不扣除
func HookValidateCapacityWithoutIncrease(ctx context.Context, fs *FileSystem) error {
	file := ctx.Value(fsctx.FileHeaderCtx).(FileHeader)
	// 驗證並扣除容量
	if fs.User.GetRemainingCapacity() < file.GetSize() {
		return ErrInsufficientCapacity
	}
	return nil
}

// HookChangeCapacity 根據原有文件和新文件的大小更新使用者容量
func HookChangeCapacity(ctx context.Context, fs *FileSystem) error {
	newFile := ctx.Value(fsctx.FileHeaderCtx).(FileHeader)
	originFile := ctx.Value(fsctx.FileModelCtx).(model.File)

	if newFile.GetSize() > originFile.Size {
		if !fs.ValidateCapacity(ctx, newFile.GetSize()-originFile.Size) {
			return ErrInsufficientCapacity
		}
		return nil
	}

	fs.User.DeductionStorage(originFile.Size - newFile.GetSize())
	return nil
}

// HookDeleteTempFile 刪除已儲存的暫存檔
func HookDeleteTempFile(ctx context.Context, fs *FileSystem) error {
	filePath := ctx.Value(fsctx.SavePathCtx).(string)
	// 刪除暫存檔
	_, err := fs.Handler.Delete(ctx, []string{filePath})
	if err != nil {
		util.Log().Warning("無法清理上傳暫存檔，%s", err)
	}

	return nil
}

// HookCleanFileContent 清空文件內容
func HookCleanFileContent(ctx context.Context, fs *FileSystem) error {
	filePath := ctx.Value(fsctx.SavePathCtx).(string)
	// 清空內容
	return fs.Handler.Put(ctx, ioutil.NopCloser(strings.NewReader("")), filePath, 0)
}

// HookClearFileSize 將原始文件的尺寸設為0
func HookClearFileSize(ctx context.Context, fs *FileSystem) error {
	originFile, ok := ctx.Value(fsctx.FileModelCtx).(model.File)
	if !ok {
		return ErrObjectNotExist
	}
	return originFile.UpdateSize(0)
}

// HookCancelContext 取消上下文
func HookCancelContext(ctx context.Context, fs *FileSystem) error {
	cancelFunc, ok := ctx.Value(fsctx.CancelFuncCtx).(context.CancelFunc)
	if ok {
		cancelFunc()
	}
	return nil
}

// HookGiveBackCapacity 歸還使用者容量
func HookGiveBackCapacity(ctx context.Context, fs *FileSystem) error {
	file := ctx.Value(fsctx.FileHeaderCtx).(FileHeader)
	once, ok := ctx.Value(fsctx.ValidateCapacityOnceCtx).(*sync.Once)
	if !ok {
		once = &sync.Once{}
	}

	// 歸還使用者容量
	res := true
	once.Do(func() {
		res = fs.User.DeductionStorage(file.GetSize())
	})

	if !res {
		return errors.New("無法繼續降低使用者已用儲存")
	}
	return nil
}

// HookUpdateSourceName 更新文件SourceName
// TODO：測試
func HookUpdateSourceName(ctx context.Context, fs *FileSystem) error {
	originFile, ok := ctx.Value(fsctx.FileModelCtx).(model.File)
	if !ok {
		return ErrObjectNotExist
	}
	return originFile.UpdateSourceName(originFile.SourceName)
}

// GenericAfterUpdate 文件內容更新後
func GenericAfterUpdate(ctx context.Context, fs *FileSystem) error {
	// 更新文件尺寸
	originFile, ok := ctx.Value(fsctx.FileModelCtx).(model.File)
	if !ok {
		return ErrObjectNotExist
	}

	fs.SetTargetFile(&[]model.File{originFile})

	newFile, ok := ctx.Value(fsctx.FileHeaderCtx).(FileHeader)
	if !ok {
		return ErrObjectNotExist
	}
	err := originFile.UpdateSize(newFile.GetSize())
	if err != nil {
		return err
	}

	// 嘗試清空原有縮圖並重新生成
	if originFile.GetPolicy().IsThumbGenerateNeeded() {
		fs.recycleLock.Lock()
		go func() {
			defer fs.recycleLock.Unlock()
			if originFile.PicInfo != "" {
				_, _ = fs.Handler.Delete(ctx, []string{originFile.SourceName + conf.ThumbConfig.FileSuffix})
				fs.GenerateThumbnail(ctx, &originFile)
			}
		}()
	}

	return nil
}

// SlaveAfterUpload Slave模式下上傳完成鉤子
func SlaveAfterUpload(ctx context.Context, fs *FileSystem) error {
	fileHeader := ctx.Value(fsctx.FileHeaderCtx).(FileHeader)
	policy := ctx.Value(fsctx.UploadPolicyCtx).(serializer.UploadPolicy)

	// 構造一個model.File，用於生成縮圖
	file := model.File{
		Name:       fileHeader.GetFileName(),
		SourceName: ctx.Value(fsctx.SavePathCtx).(string),
	}
	fs.GenerateThumbnail(ctx, &file)

	if policy.CallbackURL == "" {
		return nil
	}

	// 發送回調請求
	callbackBody := serializer.UploadCallback{
		Name:       file.Name,
		SourceName: file.SourceName,
		PicInfo:    file.PicInfo,
		Size:       fileHeader.GetSize(),
	}
	return request.RemoteCallback(policy.CallbackURL, callbackBody)
}

// GenericAfterUpload 文件上傳完成後，包含資料庫操作
func GenericAfterUpload(ctx context.Context, fs *FileSystem) error {
	// 文件存放的虛擬路徑
	virtualPath := ctx.Value(fsctx.FileHeaderCtx).(FileHeader).GetVirtualPath()

	// 檢查路徑是否存在，不存在就建立
	isExist, folder := fs.IsPathExist(virtualPath)
	if !isExist {
		newFolder, err := fs.CreateDirectory(ctx, virtualPath)
		if err != nil {
			return err
		}
		folder = newFolder
	}

	// 檢查文件是否存在
	if ok, _ := fs.IsChildFileExist(
		folder,
		ctx.Value(fsctx.FileHeaderCtx).(FileHeader).GetFileName(),
	); ok {
		return ErrFileExisted
	}

	// 向資料庫中插入記錄
	file, err := fs.AddFile(ctx, folder)
	if err != nil {
		return ErrInsertFileRecord
	}
	fs.SetTargetFile(&[]model.File{*file})

	// 非同步嘗試生成縮圖
	if fs.User.Policy.IsThumbGenerateNeeded() {
		fs.recycleLock.Lock()
		go func() {
			defer fs.recycleLock.Unlock()
			fs.GenerateThumbnail(ctx, file)
		}()
	}

	return nil
}
