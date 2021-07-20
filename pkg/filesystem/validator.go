package filesystem

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/cloudreve/Cloudreve/v3/pkg/util"
)

/* ==========
	 驗證器
   ==========
*/

// 文件/路徑名保留字元
var reservedCharacter = []string{"\\", "?", "*", "<", "\"", ":", ">", "/", "|"}

// ValidateLegalName 驗證檔案名/資料夾名是否合法
func (fs *FileSystem) ValidateLegalName(ctx context.Context, name string) bool {
	// 是否包含保留字元
	for _, value := range reservedCharacter {
		if strings.Contains(name, value) {
			return false
		}
	}

	// 是否超出長度限制
	if len(name) >= 256 {
		return false
	}

	// 是否為空限制
	if len(name) == 0 {
		return false
	}

	// 結尾不能是空格
	if strings.HasSuffix(name, " ") {
		return false
	}

	return true
}

// ValidateFileSize 驗證上傳的檔案大小是否超出限制
func (fs *FileSystem) ValidateFileSize(ctx context.Context, size uint64) bool {
	if fs.User.Policy.MaxSize == 0 {
		return true
	}
	return size <= fs.User.Policy.MaxSize
}

// ValidateCapacity 驗證並扣除使用者容量
func (fs *FileSystem) ValidateCapacity(ctx context.Context, size uint64) bool {
	return fs.User.IncreaseStorage(size)
}

// ValidateExtension 驗證文件副檔名
func (fs *FileSystem) ValidateExtension(ctx context.Context, fileName string) bool {
	// 不需要驗證
	if len(fs.User.Policy.OptionsSerialized.FileType) == 0 {
		return true
	}

	return IsInExtensionList(fs.User.Policy.OptionsSerialized.FileType, fileName)
}

// IsInExtensionList 返回文件的副檔名是否在給定的列表範圍內
func IsInExtensionList(extList []string, fileName string) bool {
	ext := strings.ToLower(filepath.Ext(fileName))
	// 無副檔名時
	if len(ext) == 0 {
		return false
	}

	if util.ContainsString(extList, ext[1:]) {
		return true
	}

	return false
}
