package filesystem

import (
	"path"

	model "github.com/cloudreve/Cloudreve/v3/models"
	"github.com/cloudreve/Cloudreve/v3/pkg/util"
)

/* =================
	 路徑/目錄相關
   =================
*/

// IsPathExist 返回給定目錄是否存在
// 如果存在就返回目錄
func (fs *FileSystem) IsPathExist(path string) (bool, *model.Folder) {
	pathList := util.SplitPath(path)
	if len(pathList) == 0 {
		return false, nil
	}

	// 遞迴步入目錄
	// TODO:測試新增
	var currentFolder *model.Folder

	// 如果已設定跟目錄物件，則從給定目錄向下遍歷
	if fs.Root != nil {
		currentFolder = fs.Root
	}

	for _, folderName := range pathList {
		var err error

		// 根目錄
		if folderName == "/" {
			if currentFolder != nil {
				continue
			}
			currentFolder, err = fs.User.Root()
			if err != nil {
				return false, nil
			}
		} else {
			currentFolder, err = currentFolder.GetChild(folderName)
			if err != nil {
				return false, nil
			}
		}
	}

	return true, currentFolder
}

// IsFileExist 返回給定路徑的文件是否存在
func (fs *FileSystem) IsFileExist(fullPath string) (bool, *model.File) {
	basePath := path.Dir(fullPath)
	fileName := path.Base(fullPath)

	// 獲得父目錄
	exist, parent := fs.IsPathExist(basePath)
	if !exist {
		return false, nil
	}

	file, err := parent.GetChildFile(fileName)

	return err == nil, file
}

// IsChildFileExist 確定folder目錄下是否有名為name的文件
func (fs *FileSystem) IsChildFileExist(folder *model.Folder, name string) (bool, *model.File) {
	file, err := folder.GetChildFile(name)
	return err == nil, file
}
