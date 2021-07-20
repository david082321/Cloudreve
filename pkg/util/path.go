package util

import (
	"os"
	"path"
	"path/filepath"
	"strings"
)

// DotPathToStandardPath 將","分割的路徑轉換為標準路徑
func DotPathToStandardPath(path string) string {
	return "/" + strings.Replace(path, ",", "/", -1)
}

// FillSlash 給路徑補全`/`
func FillSlash(path string) string {
	if path == "/" {
		return path
	}
	return path + "/"
}

// RemoveSlash 移除路徑最後的`/`
func RemoveSlash(path string) string {
	if len(path) > 1 {
		return strings.TrimSuffix(path, "/")
	}
	return path
}

// SplitPath 分割路徑為列表
func SplitPath(path string) []string {
	if len(path) == 0 || path[0] != '/' {
		return []string{}
	}

	if path == "/" {
		return []string{"/"}
	}

	pathSplit := strings.Split(path, "/")
	pathSplit[0] = "/"
	return pathSplit
}

// FormSlash 將path中的反斜線'\'取代為'/'
func FormSlash(old string) string {
	return path.Clean(strings.ReplaceAll(old, "\\", "/"))
}

// RelativePath 獲取相對可執行文件的路徑
func RelativePath(name string) string {
	if filepath.IsAbs(name) {
		return name
	}
	e, _ := os.Executable()
	return filepath.Join(filepath.Dir(e), name)
}
