package serializer

import (
	"encoding/gob"
	"time"
)

func init() {
	gob.Register(ObjectProps{})
}

// ObjectProps 文件、目錄物件的詳細屬性訊息
type ObjectProps struct {
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	Policy         string    `json:"policy"`
	Size           uint64    `json:"size"`
	ChildFolderNum int       `json:"child_folder_num"`
	ChildFileNum   int       `json:"child_file_num"`
	Path           string    `json:"path"`

	QueryDate time.Time `json:"query_date"`
}
