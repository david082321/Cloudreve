package response

import (
	"io"
	"time"
)

// ContentResponse 獲取文件內容類方法的通用返回值。
// 有些上傳策略需要重定向，
// 有些直接寫文件資料到瀏覽器
type ContentResponse struct {
	Redirect bool
	Content  RSCloser
	URL      string
	MaxAge   int
}

// RSCloser 儲存策略適配器返回的文件流，有些策略需要帶有Closer
type RSCloser interface {
	io.ReadSeeker
	io.Closer
}

// Object 列出文件、目錄時返回的物件
type Object struct {
	Name         string    `json:"name"`
	RelativePath string    `json:"relative_path"`
	Source       string    `json:"source"`
	Size         uint64    `json:"size"`
	IsDir        bool      `json:"is_dir"`
	LastModify   time.Time `json:"last_modify"`
}
