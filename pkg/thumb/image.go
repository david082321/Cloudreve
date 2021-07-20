package thumb

import (
	"errors"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"path/filepath"
	"strings"

	model "github.com/cloudreve/Cloudreve/v3/models"
	"github.com/cloudreve/Cloudreve/v3/pkg/util"

	"github.com/nfnt/resize"
)

// Thumb 縮圖
type Thumb struct {
	src image.Image
	ext string
}

// NewThumbFromFile 從文件資料獲取新的Thumb物件，
// 嘗試通過檔案名name解碼圖像
func NewThumbFromFile(file io.Reader, name string) (*Thumb, error) {
	ext := strings.ToLower(filepath.Ext(name))
	// 無副檔名時
	if len(ext) == 0 {
		return nil, errors.New("未知的圖像類型")
	}

	var err error
	var img image.Image
	switch ext[1:] {
	case "jpg":
		img, err = jpeg.Decode(file)
	case "jpeg":
		img, err = jpeg.Decode(file)
	case "gif":
		img, err = gif.Decode(file)
	case "png":
		img, err = png.Decode(file)
	default:
		return nil, errors.New("未知的圖像類型")
	}
	if err != nil {
		return nil, err
	}

	return &Thumb{
		src: img,
		ext: ext[1:],
	}, nil
}

// GetThumb 生成給定最大尺寸的縮圖
func (image *Thumb) GetThumb(width, height uint) {
	image.src = resize.Thumbnail(width, height, image.src, resize.Lanczos3)
}

// GetSize 獲取圖像尺寸
func (image *Thumb) GetSize() (int, int) {
	b := image.src.Bounds()
	return b.Max.X, b.Max.Y
}

// Save 儲存圖像到給定路徑
func (image *Thumb) Save(path string) (err error) {
	out, err := util.CreatNestedFile(path)

	if err != nil {
		return err
	}
	defer out.Close()

	err = png.Encode(out, image.src)
	return err

}

// CreateAvatar 建立大頭貼
func (image *Thumb) CreateAvatar(uid uint) error {
	// 讀取大頭貼相關設定
	savePath := util.RelativePath(model.GetSettingByName("avatar_path"))
	s := model.GetIntSetting("avatar_size_s", 50)
	m := model.GetIntSetting("avatar_size_m", 130)
	l := model.GetIntSetting("avatar_size_l", 200)

	// 生成大頭貼縮圖
	src := image.src
	for k, size := range []int{s, m, l} {
		image.src = resize.Resize(uint(size), uint(size), src, resize.Lanczos3)
		err := image.Save(filepath.Join(savePath, fmt.Sprintf("avatar_%d_%d.png", uid, k)))
		if err != nil {
			return err
		}
	}

	return nil

}
