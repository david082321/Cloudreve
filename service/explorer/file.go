package explorer

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	model "github.com/cloudreve/Cloudreve/v3/models"
	"github.com/cloudreve/Cloudreve/v3/pkg/cache"
	"github.com/cloudreve/Cloudreve/v3/pkg/filesystem"
	"github.com/cloudreve/Cloudreve/v3/pkg/filesystem/driver/local"
	"github.com/cloudreve/Cloudreve/v3/pkg/filesystem/fsctx"
	"github.com/cloudreve/Cloudreve/v3/pkg/serializer"
	"github.com/gin-gonic/gin"
	"github.com/jinzhu/gorm"
)

// SingleFileService 對單文件進行操作的五福，path為文件完整路徑
type SingleFileService struct {
	Path string `uri:"path" json:"path" binding:"required,min=1,max=65535"`
}

// FileIDService 透過文件ID對文件進行操作的服務
type FileIDService struct {
}

// FileAnonymousGetService 匿名（外鏈）獲取文件服務
type FileAnonymousGetService struct {
	ID   uint   `uri:"id" binding:"required,min=1"`
	Name string `uri:"name" binding:"required"`
}

// DownloadService 文件下載服務
type DownloadService struct {
	ID string `uri:"id" binding:"required"`
}

// SlaveDownloadService 從機文件下載服務
type SlaveDownloadService struct {
	PathEncoded string `uri:"path" binding:"required"`
	Name        string `uri:"name" binding:"required"`
	Speed       int    `uri:"speed" binding:"min=0"`
}

// SlaveFileService 從機單文件文件相關服務
type SlaveFileService struct {
	PathEncoded string `uri:"path" binding:"required"`
}

// SlaveFilesService 從機多文件相關服務
type SlaveFilesService struct {
	Files []string `json:"files" binding:"required,gt=0"`
}

// SlaveListService 從機列表服務
type SlaveListService struct {
	Path      string `json:"path" binding:"required,min=1,max=65535"`
	Recursive bool   `json:"recursive"`
}

// New 建立新文件
func (service *SingleFileService) Create(c *gin.Context) serializer.Response {
	// 建立文件系統
	fs, err := filesystem.NewFileSystemFromContext(c)
	if err != nil {
		return serializer.Err(serializer.CodePolicyNotAllowed, err.Error(), err)
	}
	defer fs.Recycle()

	// 上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx = context.WithValue(ctx, fsctx.DisableOverwrite, true)

	// 給文件系統分配鉤子
	fs.Use("BeforeUpload", filesystem.HookValidateFile)
	fs.Use("AfterUpload", filesystem.GenericAfterUpload)

	// 上傳空文件
	err = fs.Upload(ctx, local.FileStream{
		File:        ioutil.NopCloser(strings.NewReader("")),
		Size:        0,
		VirtualPath: path.Dir(service.Path),
		Name:        path.Base(service.Path),
	})
	if err != nil {
		return serializer.Err(serializer.CodeUploadFailed, err.Error(), err)
	}

	return serializer.Response{
		Code: 0,
	}
}

// List 列出從機上的文件
func (service *SlaveListService) List(c *gin.Context) serializer.Response {
	// 建立文件系統
	fs, err := filesystem.NewAnonymousFileSystem()
	if err != nil {
		return serializer.Err(serializer.CodePolicyNotAllowed, err.Error(), err)
	}
	defer fs.Recycle()

	objects, err := fs.Handler.List(context.Background(), service.Path, service.Recursive)
	if err != nil {
		return serializer.Err(serializer.CodeIOFailed, "無法列取文件", err)
	}

	res, _ := json.Marshal(objects)
	return serializer.Response{Data: string(res)}
}

// DownloadArchived 下載已打包的多文件
func (service *DownloadService) DownloadArchived(ctx context.Context, c *gin.Context) serializer.Response {
	// 建立文件系統
	fs, err := filesystem.NewFileSystemFromContext(c)
	if err != nil {
		return serializer.Err(serializer.CodePolicyNotAllowed, err.Error(), err)
	}
	defer fs.Recycle()

	// 尋找打包的暫存檔
	zipPath, exist := cache.Get("archive_" + service.ID)
	if !exist {
		return serializer.Err(404, "歸檔文件不存在", nil)
	}

	// 獲取文件流
	rs, err := fs.GetPhysicalFileContent(ctx, zipPath.(string))
	defer rs.Close()
	if err != nil {
		return serializer.Err(serializer.CodeNotSet, err.Error(), err)
	}

	if fs.User.Group.OptionsSerialized.OneTimeDownload {
		// 清理資源，刪除暫存檔
		_ = cache.Deletes([]string{service.ID}, "archive_")
	}

	c.Header("Content-Disposition", "attachment;")
	c.Header("Content-Type", "application/zip")
	http.ServeContent(c.Writer, c.Request, "", time.Now(), rs)

	return serializer.Response{
		Code: 0,
	}

}

// Download 簽名的匿名文件下載
func (service *FileAnonymousGetService) Download(ctx context.Context, c *gin.Context) serializer.Response {
	fs, err := filesystem.NewAnonymousFileSystem()
	if err != nil {
		return serializer.Err(serializer.CodeGroupNotAllowed, err.Error(), err)
	}
	defer fs.Recycle()

	// 尋找文件
	err = fs.SetTargetFileByIDs([]uint{service.ID})
	if err != nil {
		return serializer.Err(serializer.CodeNotSet, err.Error(), err)
	}

	// 獲取文件流
	rs, err := fs.GetDownloadContent(ctx, 0)
	defer rs.Close()
	if err != nil {
		return serializer.Err(serializer.CodeNotSet, err.Error(), err)
	}

	// 發送文件
	http.ServeContent(c.Writer, c.Request, service.Name, fs.FileTarget[0].UpdatedAt, rs)

	return serializer.Response{
		Code: 0,
	}
}

// Source 重定向到文件的有效原始連結
func (service *FileAnonymousGetService) Source(ctx context.Context, c *gin.Context) serializer.Response {
	fs, err := filesystem.NewAnonymousFileSystem()
	if err != nil {
		return serializer.Err(serializer.CodeGroupNotAllowed, err.Error(), err)
	}
	defer fs.Recycle()

	// 尋找文件
	err = fs.SetTargetFileByIDs([]uint{service.ID})
	if err != nil {
		return serializer.Err(serializer.CodeNotSet, err.Error(), err)
	}

	// 獲取文件流
	res, err := fs.SignURL(ctx, &fs.FileTarget[0],
		int64(model.GetIntSetting("preview_timeout", 60)), false)
	if err != nil {
		return serializer.Err(serializer.CodeNotSet, err.Error(), err)
	}

	return serializer.Response{
		Code: -302,
		Data: res,
	}
}

// CreateDocPreviewSession 建立DOC文件預覽工作階段，返回預覽地址
func (service *FileIDService) CreateDocPreviewSession(ctx context.Context, c *gin.Context) serializer.Response {
	// 建立文件系統
	fs, err := filesystem.NewFileSystemFromContext(c)
	if err != nil {
		return serializer.Err(serializer.CodePolicyNotAllowed, err.Error(), err)
	}
	defer fs.Recycle()

	// 獲取物件id
	objectID, _ := c.Get("object_id")

	// 如果上下文中已有File物件，則重設目標
	if file, ok := ctx.Value(fsctx.FileModelCtx).(*model.File); ok {
		fs.SetTargetFile(&[]model.File{*file})
		objectID = uint(0)
	}

	// 如果上下文中已有Folder物件，則重設根目錄
	if folder, ok := ctx.Value(fsctx.FolderModelCtx).(*model.Folder); ok {
		fs.Root = folder
		path := ctx.Value(fsctx.PathCtx).(string)
		err := fs.ResetFileIfNotExist(ctx, path)
		if err != nil {
			return serializer.Err(serializer.CodeNotFound, err.Error(), err)
		}
		objectID = uint(0)
	}

	// 獲取文件臨時下載網址
	downloadURL, err := fs.GetDownloadURL(ctx, objectID.(uint), "doc_preview_timeout")
	if err != nil {
		return serializer.Err(serializer.CodeNotSet, err.Error(), err)
	}

	// 生成最終的預覽器地址
	// TODO 從配置檔案中讀取
	viewerBase, _ := url.Parse("https://view.officeapps.live.com/op/view.aspx")
	params := viewerBase.Query()
	params.Set("src", downloadURL)
	viewerBase.RawQuery = params.Encode()

	return serializer.Response{
		Code: 0,
		Data: viewerBase.String(),
	}
}

// CreateDownloadSession 建立下載工作階段，獲取下載URL
func (service *FileIDService) CreateDownloadSession(ctx context.Context, c *gin.Context) serializer.Response {
	// 建立文件系統
	fs, err := filesystem.NewFileSystemFromContext(c)
	if err != nil {
		return serializer.Err(serializer.CodePolicyNotAllowed, err.Error(), err)
	}
	defer fs.Recycle()

	// 獲取物件id
	objectID, _ := c.Get("object_id")

	// 獲取下載網址
	downloadURL, err := fs.GetDownloadURL(ctx, objectID.(uint), "download_timeout")
	if err != nil {
		return serializer.Err(serializer.CodeNotSet, err.Error(), err)
	}

	return serializer.Response{
		Code: 0,
		Data: downloadURL,
	}
}

// Download 透過簽名URL的文件下載，無需登入
func (service *DownloadService) Download(ctx context.Context, c *gin.Context) serializer.Response {
	// 建立文件系統
	fs, err := filesystem.NewFileSystemFromContext(c)
	if err != nil {
		return serializer.Err(serializer.CodePolicyNotAllowed, err.Error(), err)
	}
	defer fs.Recycle()

	// 尋找打包的暫存檔
	file, exist := cache.Get("download_" + service.ID)
	if !exist {
		return serializer.Err(404, "文件下載工作階段不存在", nil)
	}
	fs.FileTarget = []model.File{file.(model.File)}

	// 開始處理下載
	ctx = context.WithValue(ctx, fsctx.GinCtx, c)
	rs, err := fs.GetDownloadContent(ctx, 0)
	if err != nil {
		return serializer.Err(serializer.CodeNotSet, err.Error(), err)
	}
	defer rs.Close()

	// 設定檔案名
	c.Header("Content-Disposition", "attachment; filename=\""+url.PathEscape(fs.FileTarget[0].Name)+"\"")

	if fs.User.Group.OptionsSerialized.OneTimeDownload {
		// 清理資源，刪除暫存檔
		_ = cache.Deletes([]string{service.ID}, "download_")
	}

	// 發送文件
	http.ServeContent(c.Writer, c.Request, fs.FileTarget[0].Name, fs.FileTarget[0].UpdatedAt, rs)

	return serializer.Response{
		Code: 0,
	}
}

// PreviewContent 預覽文件，需要登入工作階段, isText - 是否為文字文件，文字文件會
// 強制經由服務端中轉
func (service *FileIDService) PreviewContent(ctx context.Context, c *gin.Context, isText bool) serializer.Response {
	// 建立文件系統
	fs, err := filesystem.NewFileSystemFromContext(c)
	if err != nil {
		return serializer.Err(serializer.CodePolicyNotAllowed, err.Error(), err)
	}
	defer fs.Recycle()

	// 獲取物件id
	objectID, _ := c.Get("object_id")

	// 如果上下文中已有File物件，則重設目標
	if file, ok := ctx.Value(fsctx.FileModelCtx).(*model.File); ok {
		fs.SetTargetFile(&[]model.File{*file})
		objectID = uint(0)
	}

	// 如果上下文中已有Folder物件，則重設根目錄
	if folder, ok := ctx.Value(fsctx.FolderModelCtx).(*model.Folder); ok {
		fs.Root = folder
		path := ctx.Value(fsctx.PathCtx).(string)
		err := fs.ResetFileIfNotExist(ctx, path)
		if err != nil {
			return serializer.Err(serializer.CodeNotFound, err.Error(), err)
		}
		objectID = uint(0)
	}

	// 獲取文件預覽響應
	resp, err := fs.Preview(ctx, objectID.(uint), isText)
	if err != nil {
		return serializer.Err(serializer.CodeNotSet, err.Error(), err)
	}

	// 重定向到文件源
	if resp.Redirect {
		c.Header("Cache-Control", fmt.Sprintf("max-age=%d", resp.MaxAge))
		return serializer.Response{
			Code: -301,
			Data: resp.URL,
		}
	}

	// 直接返回文件內容
	defer resp.Content.Close()

	if isText {
		c.Header("Cache-Control", "no-cache")
	}

	http.ServeContent(c.Writer, c.Request, fs.FileTarget[0].Name, fs.FileTarget[0].UpdatedAt, resp.Content)

	return serializer.Response{
		Code: 0,
	}
}

// PutContent 更新文件內容
func (service *FileIDService) PutContent(ctx context.Context, c *gin.Context) serializer.Response {
	// 建立上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 取得檔案大小
	fileSize, err := strconv.ParseUint(c.Request.Header.Get("Content-Length"), 10, 64)
	if err != nil {

		return serializer.ParamErr("無法解析文件尺寸", err)
	}

	fileData := local.FileStream{
		MIMEType: c.Request.Header.Get("Content-Type"),
		File:     c.Request.Body,
		Size:     fileSize,
	}

	// 建立文件系統
	fs, err := filesystem.NewFileSystemFromContext(c)
	if err != nil {
		return serializer.Err(serializer.CodePolicyNotAllowed, err.Error(), err)
	}
	uploadCtx := context.WithValue(ctx, fsctx.GinCtx, c)

	// 取得現有文件
	fileID, _ := c.Get("object_id")
	originFile, _ := model.GetFilesByIDs([]uint{fileID.(uint)}, fs.User.ID)
	if len(originFile) == 0 {
		return serializer.Err(404, "文件不存在", nil)
	}
	fileData.Name = originFile[0].Name

	// 檢查此文件是否有軟連結
	fileList, err := model.RemoveFilesWithSoftLinks([]model.File{originFile[0]})
	if err == nil && len(fileList) == 0 {
		// 如果包含軟連接，應重新生成新文件副本，並更新source_name
		originFile[0].SourceName = fs.GenerateSavePath(uploadCtx, fileData)
		fs.Use("AfterUpload", filesystem.HookUpdateSourceName)
		fs.Use("AfterUploadCanceled", filesystem.HookUpdateSourceName)
		fs.Use("AfterValidateFailed", filesystem.HookUpdateSourceName)
	}

	// 給文件系統分配鉤子
	fs.Use("BeforeUpload", filesystem.HookResetPolicy)
	fs.Use("BeforeUpload", filesystem.HookValidateFile)
	fs.Use("BeforeUpload", filesystem.HookChangeCapacity)
	fs.Use("AfterUploadCanceled", filesystem.HookCleanFileContent)
	fs.Use("AfterUploadCanceled", filesystem.HookClearFileSize)
	fs.Use("AfterUploadCanceled", filesystem.HookGiveBackCapacity)
	fs.Use("AfterUpload", filesystem.GenericAfterUpdate)
	fs.Use("AfterValidateFailed", filesystem.HookCleanFileContent)
	fs.Use("AfterValidateFailed", filesystem.HookClearFileSize)
	fs.Use("AfterValidateFailed", filesystem.HookGiveBackCapacity)

	// 執行上傳
	uploadCtx = context.WithValue(uploadCtx, fsctx.FileModelCtx, originFile[0])
	err = fs.Upload(uploadCtx, fileData)
	if err != nil {
		return serializer.Err(serializer.CodeUploadFailed, err.Error(), err)
	}

	return serializer.Response{
		Code: 0,
	}
}

// ServeFile 透過簽名的URL下載從機文件
func (service *SlaveDownloadService) ServeFile(ctx context.Context, c *gin.Context, isDownload bool) serializer.Response {
	// 建立文件系統
	fs, err := filesystem.NewAnonymousFileSystem()
	if err != nil {
		return serializer.Err(serializer.CodePolicyNotAllowed, err.Error(), err)
	}
	defer fs.Recycle()

	// 解碼文件路徑
	fileSource, err := base64.RawURLEncoding.DecodeString(service.PathEncoded)
	if err != nil {
		return serializer.ParamErr("無法解析的文件地址", err)
	}

	// 根據URL裡的訊息建立一個文件物件和使用者物件
	file := model.File{
		Name:       service.Name,
		SourceName: string(fileSource),
		Policy: model.Policy{
			Model: gorm.Model{ID: 1},
			Type:  "local",
		},
	}
	fs.User = &model.User{
		Group: model.Group{SpeedLimit: service.Speed},
	}
	fs.FileTarget = []model.File{file}

	// 開始處理下載
	ctx = context.WithValue(ctx, fsctx.GinCtx, c)
	rs, err := fs.GetDownloadContent(ctx, 0)
	if err != nil {
		return serializer.Err(serializer.CodeNotSet, err.Error(), err)
	}
	defer rs.Close()

	// 設定下載檔案名
	if isDownload {
		c.Header("Content-Disposition", "attachment; filename=\""+url.PathEscape(fs.FileTarget[0].Name)+"\"")
	}

	// 發送文件
	http.ServeContent(c.Writer, c.Request, fs.FileTarget[0].Name, time.Now(), rs)

	return serializer.Response{
		Code: 0,
	}
}

// Delete 透過簽名的URL刪除從機文件
func (service *SlaveFilesService) Delete(ctx context.Context, c *gin.Context) serializer.Response {
	// 建立文件系統
	fs, err := filesystem.NewAnonymousFileSystem()
	if err != nil {
		return serializer.Err(serializer.CodePolicyNotAllowed, err.Error(), err)
	}
	defer fs.Recycle()

	// 刪除文件
	failed, err := fs.Handler.Delete(ctx, service.Files)
	if err != nil {
		// 將Data欄位寫為字串方便主控端解析
		data, _ := json.Marshal(serializer.RemoteDeleteRequest{Files: failed})

		return serializer.Response{
			Code:  serializer.CodeNotFullySuccess,
			Data:  string(data),
			Msg:   fmt.Sprintf("有 %d 個文件未能成功刪除", len(failed)),
			Error: err.Error(),
		}
	}
	return serializer.Response{Code: 0}
}

// Thumb 透過簽名URL獲取從機文件縮圖
func (service *SlaveFileService) Thumb(ctx context.Context, c *gin.Context) serializer.Response {
	// 建立文件系統
	fs, err := filesystem.NewAnonymousFileSystem()
	if err != nil {
		return serializer.Err(serializer.CodePolicyNotAllowed, err.Error(), err)
	}
	defer fs.Recycle()

	// 解碼文件路徑
	fileSource, err := base64.RawURLEncoding.DecodeString(service.PathEncoded)
	if err != nil {
		return serializer.ParamErr("無法解析的文件地址", err)
	}
	fs.FileTarget = []model.File{{SourceName: string(fileSource), PicInfo: "1,1"}}

	// 獲取縮圖
	resp, err := fs.GetThumb(ctx, 0)
	if err != nil {
		return serializer.Err(serializer.CodeNotSet, "無法獲取縮圖", err)
	}

	defer resp.Content.Close()
	http.ServeContent(c.Writer, c.Request, "thumb.png", time.Now(), resp.Content)

	return serializer.Response{Code: 0}
}
