package callback

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudreve/Cloudreve/v3/pkg/filesystem"
	"github.com/cloudreve/Cloudreve/v3/pkg/filesystem/driver/cos"
	"github.com/cloudreve/Cloudreve/v3/pkg/filesystem/driver/local"
	"github.com/cloudreve/Cloudreve/v3/pkg/filesystem/driver/onedrive"
	"github.com/cloudreve/Cloudreve/v3/pkg/filesystem/driver/s3"
	"github.com/cloudreve/Cloudreve/v3/pkg/filesystem/fsctx"
	"github.com/cloudreve/Cloudreve/v3/pkg/serializer"
	"github.com/cloudreve/Cloudreve/v3/pkg/util"
	"github.com/gin-gonic/gin"
)

// CallbackProcessService 上傳請求回調正文介面
type CallbackProcessService interface {
	GetBody(*serializer.UploadSession) serializer.UploadCallback
}

// RemoteUploadCallbackService 遠端儲存上傳回調請求服務
type RemoteUploadCallbackService struct {
	Data serializer.UploadCallback `json:"data" binding:"required"`
}

// GetBody 返回回調正文
func (service RemoteUploadCallbackService) GetBody(session *serializer.UploadSession) serializer.UploadCallback {
	return service.Data
}

// UploadCallbackService OOS/七牛雲端儲存上傳回調請求服務
type UploadCallbackService struct {
	Name       string `json:"name"`
	SourceName string `json:"source_name"`
	PicInfo    string `json:"pic_info"`
	Size       uint64 `json:"size"`
}

// UpyunCallbackService 又拍雲上傳回調請求服務
type UpyunCallbackService struct {
	Code       int    `form:"code" binding:"required"`
	Message    string `form:"message" binding:"required"`
	SourceName string `form:"url" binding:"required"`
	Width      string `form:"image-width"`
	Height     string `form:"image-height"`
	Size       uint64 `form:"file_size"`
}

// OneDriveCallback OneDrive 用戶端回調正文
type OneDriveCallback struct {
	ID   string `json:"id" binding:"required"`
	Meta *onedrive.FileInfo
}

// COSCallback COS 用戶端回調正文
type COSCallback struct {
	Bucket string `form:"bucket"`
	Etag   string `form:"etag"`
}

// S3Callback S3 用戶端回調正文
type S3Callback struct {
	Bucket string `form:"bucket"`
	Etag   string `form:"etag"`
	Key    string `form:"key"`
}

// GetBody 返回回調正文
func (service UpyunCallbackService) GetBody(session *serializer.UploadSession) serializer.UploadCallback {
	res := serializer.UploadCallback{
		Name:       session.Name,
		SourceName: service.SourceName,
		Size:       service.Size,
	}
	if service.Width != "" {
		res.PicInfo = service.Width + "," + service.Height
	}

	return res
}

// GetBody 返回回調正文
func (service UploadCallbackService) GetBody(session *serializer.UploadSession) serializer.UploadCallback {
	return serializer.UploadCallback{
		Name:       service.Name,
		SourceName: service.SourceName,
		PicInfo:    service.PicInfo,
		Size:       service.Size,
	}
}

// GetBody 返回回調正文
func (service OneDriveCallback) GetBody(session *serializer.UploadSession) serializer.UploadCallback {
	var picInfo = "0,0"
	if service.Meta.Image.Width != 0 {
		picInfo = fmt.Sprintf("%d,%d", service.Meta.Image.Width, service.Meta.Image.Height)
	}
	return serializer.UploadCallback{
		Name:       session.Name,
		SourceName: session.SavePath,
		PicInfo:    picInfo,
		Size:       session.Size,
	}
}

// GetBody 返回回調正文
func (service COSCallback) GetBody(session *serializer.UploadSession) serializer.UploadCallback {
	return serializer.UploadCallback{
		Name:       session.Name,
		SourceName: session.SavePath,
		PicInfo:    "",
		Size:       session.Size,
	}
}

// GetBody 返回回調正文
func (service S3Callback) GetBody(session *serializer.UploadSession) serializer.UploadCallback {
	return serializer.UploadCallback{
		Name:       session.Name,
		SourceName: session.SavePath,
		PicInfo:    "",
		Size:       session.Size,
	}
}

// ProcessCallback 處理上傳結果回調
func ProcessCallback(service CallbackProcessService, c *gin.Context) serializer.Response {
	// 建立文件系統
	fs, err := filesystem.NewFileSystemFromCallback(c)
	if err != nil {
		return serializer.Err(serializer.CodePolicyNotAllowed, err.Error(), err)
	}
	defer fs.Recycle()

	// 獲取回調工作階段
	callbackSessionRaw, _ := c.Get("callbackSession")
	callbackSession := callbackSessionRaw.(*serializer.UploadSession)
	callbackBody := service.GetBody(callbackSession)

	// 獲取父目錄
	exist, parentFolder := fs.IsPathExist(callbackSession.VirtualPath)
	if !exist {
		newFolder, err := fs.CreateDirectory(context.Background(), callbackSession.VirtualPath)
		if err != nil {
			return serializer.Err(serializer.CodeParamErr, "指定目錄不存在", err)
		}
		parentFolder = newFolder
	}

	// 建立文件頭
	fileHeader := local.FileStream{
		Size:        callbackBody.Size,
		VirtualPath: callbackSession.VirtualPath,
		Name:        callbackSession.Name,
	}

	// 生成上下文
	ctx := context.WithValue(context.Background(), fsctx.FileHeaderCtx, fileHeader)
	ctx = context.WithValue(ctx, fsctx.SavePathCtx, callbackBody.SourceName)

	// 添加鉤子
	fs.Use("BeforeAddFile", filesystem.HookValidateFile)
	fs.Use("BeforeAddFile", filesystem.HookValidateCapacity)
	fs.Use("AfterValidateFailed", filesystem.HookGiveBackCapacity)
	fs.Use("AfterValidateFailed", filesystem.HookDeleteTempFile)
	fs.Use("BeforeAddFileFailed", filesystem.HookDeleteTempFile)

	// 向資料庫中添加文件
	file, err := fs.AddFile(ctx, parentFolder)
	if err != nil {
		return serializer.Err(serializer.CodeUploadFailed, err.Error(), err)
	}

	// 如果是圖片，則更新圖片訊息
	if callbackBody.PicInfo != "" {
		if err := file.UpdatePicInfo(callbackBody.PicInfo); err != nil {
			util.Log().Debug("無法更新回調文件的圖片訊息：%s", err)
		}
	}

	return serializer.Response{
		Code: 0,
	}
}

// PreProcess 對OneDrive用戶端回調進行預處理驗證
func (service *OneDriveCallback) PreProcess(c *gin.Context) serializer.Response {
	// 建立文件系統
	fs, err := filesystem.NewFileSystemFromCallback(c)
	if err != nil {
		return serializer.Err(serializer.CodePolicyNotAllowed, err.Error(), err)
	}
	defer fs.Recycle()

	// 獲取回調工作階段
	callbackSessionRaw, _ := c.Get("callbackSession")
	callbackSession := callbackSessionRaw.(*serializer.UploadSession)

	// 獲取文件訊息
	info, err := fs.Handler.(onedrive.Driver).Client.Meta(context.Background(), service.ID, "")
	if err != nil {
		return serializer.Err(serializer.CodeUploadFailed, "文件元訊息查詢失敗", err)
	}

	// 驗證與回調工作階段中是否一致
	actualPath := strings.TrimPrefix(callbackSession.SavePath, "/")
	isSizeCheckFailed := callbackSession.Size != info.Size

	// SharePoint 會對 Office 文件增加 meta data 導致檔案大小不一致，這裡增加 10 KB 寬容
	// See: https://github.com/OneDrive/onedrive-api-docs/issues/935
	if strings.Contains(fs.Policy.OptionsSerialized.OdDriver, "sharepoint.com") && isSizeCheckFailed && (info.Size > callbackSession.Size) && (info.Size-callbackSession.Size <= 10240) {
		isSizeCheckFailed = false
	}

	if isSizeCheckFailed || info.GetSourcePath() != actualPath {
		fs.Handler.(onedrive.Driver).Client.Delete(context.Background(), []string{info.GetSourcePath()})
		return serializer.Err(serializer.CodeUploadFailed, "文件訊息不一致", err)
	}
	service.Meta = info
	return ProcessCallback(service, c)
}

// PreProcess 對COS用戶端回調進行預處理
func (service *COSCallback) PreProcess(c *gin.Context) serializer.Response {
	// 建立文件系統
	fs, err := filesystem.NewFileSystemFromCallback(c)
	if err != nil {
		return serializer.Err(serializer.CodePolicyNotAllowed, err.Error(), err)
	}
	defer fs.Recycle()

	// 獲取回調工作階段
	callbackSessionRaw, _ := c.Get("callbackSession")
	callbackSession := callbackSessionRaw.(*serializer.UploadSession)

	// 獲取文件訊息
	info, err := fs.Handler.(cos.Driver).Meta(context.Background(), callbackSession.SavePath)
	if err != nil {
		return serializer.Err(serializer.CodeUploadFailed, "文件訊息不一致", err)
	}

	// 驗證實際文件訊息與回調工作階段中是否一致
	if callbackSession.Size != info.Size || callbackSession.Key != info.CallbackKey {
		return serializer.Err(serializer.CodeUploadFailed, "文件訊息不一致", err)
	}

	return ProcessCallback(service, c)
}

// PreProcess 對S3用戶端回調進行預處理
func (service *S3Callback) PreProcess(c *gin.Context) serializer.Response {
	// 建立文件系統
	fs, err := filesystem.NewFileSystemFromCallback(c)
	if err != nil {
		return serializer.Err(serializer.CodePolicyNotAllowed, err.Error(), err)
	}
	defer fs.Recycle()

	// 獲取回調工作階段
	callbackSessionRaw, _ := c.Get("callbackSession")
	callbackSession := callbackSessionRaw.(*serializer.UploadSession)

	// 獲取文件訊息
	info, err := fs.Handler.(s3.Driver).Meta(context.Background(), callbackSession.SavePath)
	if err != nil {
		return serializer.Err(serializer.CodeUploadFailed, "文件訊息不一致", err)
	}

	// 驗證實際文件訊息與回調工作階段中是否一致
	if callbackSession.Size != info.Size || service.Etag != info.Etag {
		return serializer.Err(serializer.CodeUploadFailed, "文件訊息不一致", err)
	}

	return ProcessCallback(service, c)
}
