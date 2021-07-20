package filesystem

import (
	"context"
	"fmt"
	"strconv"

	model "github.com/cloudreve/Cloudreve/v3/models"
	"github.com/cloudreve/Cloudreve/v3/pkg/conf"
	"github.com/cloudreve/Cloudreve/v3/pkg/filesystem/fsctx"
	"github.com/cloudreve/Cloudreve/v3/pkg/filesystem/response"
	"github.com/cloudreve/Cloudreve/v3/pkg/thumb"
	"github.com/cloudreve/Cloudreve/v3/pkg/util"
)

/* ================
     圖像處理相關
   ================
*/

// HandledExtension 可以生成縮圖的文件副檔名
var HandledExtension = []string{"jpg", "jpeg", "png", "gif"}

// GetThumb 獲取文件的縮圖
func (fs *FileSystem) GetThumb(ctx context.Context, id uint) (*response.ContentResponse, error) {
	// 根據 ID 尋找文件
	err := fs.resetFileIDIfNotExist(ctx, id)
	if err != nil || fs.FileTarget[0].PicInfo == "" {
		return &response.ContentResponse{
			Redirect: false,
		}, ErrObjectNotExist
	}

	w, h := fs.GenerateThumbnailSize(0, 0)
	ctx = context.WithValue(ctx, fsctx.ThumbSizeCtx, [2]uint{w, h})
	ctx = context.WithValue(ctx, fsctx.FileModelCtx, fs.FileTarget[0])
	res, err := fs.Handler.Thumb(ctx, fs.FileTarget[0].SourceName)
	if err == nil && conf.SystemConfig.Mode == "master" {
		res.MaxAge = model.GetIntSetting("preview_timeout", 60)
	}

	// 本機儲存策略出錯時重新生成縮圖
	if err != nil && fs.Policy.Type == "local" {
		fs.GenerateThumbnail(ctx, &fs.FileTarget[0])
	}

	return res, err
}

// GenerateThumbnail 嘗試為本機策略文件生成縮圖並獲取圖像原始大小
// TODO 失敗時，如果之前還有圖像訊息，則清除
func (fs *FileSystem) GenerateThumbnail(ctx context.Context, file *model.File) {
	// 判斷是否可以生成縮圖
	if !IsInExtensionList(HandledExtension, file.Name) {
		return
	}

	// 建立上下文
	newCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 獲取文件資料
	source, err := fs.Handler.Get(newCtx, file.SourceName)
	if err != nil {
		return
	}
	defer source.Close()

	image, err := thumb.NewThumbFromFile(source, file.Name)
	if err != nil {
		util.Log().Warning("生成縮圖時無法解析 [%s] 圖像資料：%s", file.SourceName, err)
		return
	}

	// 獲取原始圖像尺寸
	w, h := image.GetSize()

	// 生成縮圖
	image.GetThumb(fs.GenerateThumbnailSize(w, h))
	// 儲存到文件
	err = image.Save(util.RelativePath(file.SourceName + conf.ThumbConfig.FileSuffix))
	if err != nil {
		util.Log().Warning("無法儲存縮圖：%s", err)
		return
	}

	// 更新文件的圖像訊息
	if file.Model.ID > 0 {
		err = file.UpdatePicInfo(fmt.Sprintf("%d,%d", w, h))
	} else {
		file.PicInfo = fmt.Sprintf("%d,%d", w, h)
	}

	// 失敗時刪除縮圖文件
	if err != nil {
		_, _ = fs.Handler.Delete(newCtx, []string{file.SourceName + conf.ThumbConfig.FileSuffix})
	}
}

// GenerateThumbnailSize 獲取要生成的縮圖的尺寸
func (fs *FileSystem) GenerateThumbnailSize(w, h int) (uint, uint) {
	if conf.SystemConfig.Mode == "master" {
		options := model.GetSettingByNames("thumb_width", "thumb_height")
		w, _ := strconv.ParseUint(options["thumb_width"], 10, 32)
		h, _ := strconv.ParseUint(options["thumb_height"], 10, 32)
		return uint(w), uint(h)
	}
	return conf.ThumbConfig.MaxWidth, conf.ThumbConfig.MaxHeight
}
