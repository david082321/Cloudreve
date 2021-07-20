package filesystem

import (
	"context"
	"fmt"
	"path"
	"strings"
	"time"

	model "github.com/cloudreve/Cloudreve/v3/models"
	"github.com/cloudreve/Cloudreve/v3/pkg/filesystem/fsctx"
	"github.com/cloudreve/Cloudreve/v3/pkg/hashid"
	"github.com/cloudreve/Cloudreve/v3/pkg/serializer"
	"github.com/cloudreve/Cloudreve/v3/pkg/util"
)

/* =================
	 文件/目錄管理
   =================
*/

// Object 文件或者目錄
type Object struct {
	ID   string    `json:"id"`
	Name string    `json:"name"`
	Path string    `json:"path"`
	Pic  string    `json:"pic"`
	Size uint64    `json:"size"`
	Type string    `json:"type"`
	Date time.Time `json:"date"`
	Key  string    `json:"key,omitempty"`
}

// Rename 重新命名物件
func (fs *FileSystem) Rename(ctx context.Context, dir, file []uint, new string) (err error) {
	// 驗證新名字
	if !fs.ValidateLegalName(ctx, new) || (len(file) > 0 && !fs.ValidateExtension(ctx, new)) {
		return ErrIllegalObjectName
	}

	// 如果源物件是文件
	if len(file) > 0 {
		fileObject, err := model.GetFilesByIDs([]uint{file[0]}, fs.User.ID)
		if err != nil || len(fileObject) == 0 {
			return ErrPathNotExist
		}

		err = fileObject[0].Rename(new)
		if err != nil {
			return ErrFileExisted
		}
		return nil
	}

	if len(dir) > 0 {
		folderObject, err := model.GetFoldersByIDs([]uint{dir[0]}, fs.User.ID)
		if err != nil || len(folderObject) == 0 {
			return ErrPathNotExist
		}

		err = folderObject[0].Rename(new)
		if err != nil {
			return ErrFileExisted
		}
		return nil
	}

	return ErrPathNotExist
}

// Copy 複製src目錄下的文件或目錄到dst，
// 暫時只支援單文件
func (fs *FileSystem) Copy(ctx context.Context, dirs, files []uint, src, dst string) error {
	// 獲取目的目錄
	isDstExist, dstFolder := fs.IsPathExist(dst)
	isSrcExist, srcFolder := fs.IsPathExist(src)
	// 不存在時返回空的結果
	if !isDstExist || !isSrcExist {
		return ErrPathNotExist
	}

	// 記錄複製的文件的總容量
	var newUsedStorage uint64

	// 複製目錄
	if len(dirs) > 0 {
		subFileSizes, err := srcFolder.CopyFolderTo(dirs[0], dstFolder)
		if err != nil {
			return serializer.NewError(serializer.CodeDBError, "操作失敗，可能有重名衝突", err)
		}
		newUsedStorage += subFileSizes
	}

	// 複製文件
	if len(files) > 0 {
		subFileSizes, err := srcFolder.MoveOrCopyFileTo(files, dstFolder, true)
		if err != nil {
			return serializer.NewError(serializer.CodeDBError, "操作失敗，可能有重名衝突", err)
		}
		newUsedStorage += subFileSizes
	}

	// 扣除容量
	fs.User.IncreaseStorageWithoutCheck(newUsedStorage)

	return nil
}

// Move 移動文件和目錄, 將id列表dirs和files從src移動至dst
func (fs *FileSystem) Move(ctx context.Context, dirs, files []uint, src, dst string) error {
	// 獲取目的目錄
	isDstExist, dstFolder := fs.IsPathExist(dst)
	isSrcExist, srcFolder := fs.IsPathExist(src)
	// 不存在時返回空的結果
	if !isDstExist || !isSrcExist {
		return ErrPathNotExist
	}

	// 處理目錄及子文件移動
	err := srcFolder.MoveFolderTo(dirs, dstFolder)
	if err != nil {
		return serializer.NewError(serializer.CodeDBError, "操作失敗，可能有重名衝突", err)
	}

	// 處理文件移動
	_, err = srcFolder.MoveOrCopyFileTo(files, dstFolder, false)
	if err != nil {
		return serializer.NewError(serializer.CodeDBError, "操作失敗，可能有重名衝突", err)
	}

	// 移動文件

	return err
}

// Delete 遞迴刪除物件, force 為 true 時強制刪除文件記錄，忽略物理刪除是否成功
func (fs *FileSystem) Delete(ctx context.Context, dirs, files []uint, force bool) error {
	// 已刪除的總容量,map用於去重
	var deletedStorage = make(map[uint]uint64)
	var totalStorage = make(map[uint]uint64)
	// 已刪除的文件ID
	var deletedFileIDs = make([]uint, 0, len(fs.FileTarget))
	// 刪除失敗的文件的父目錄ID

	// 所有文件的ID
	var allFileIDs = make([]uint, 0, len(fs.FileTarget))

	// 列出要刪除的目錄
	if len(dirs) > 0 {
		err := fs.ListDeleteDirs(ctx, dirs)
		if err != nil {
			return err
		}
	}

	// 列出要刪除的文件
	if len(files) > 0 {
		err := fs.ListDeleteFiles(ctx, files)
		if err != nil {
			return err
		}
	}

	// 去除待刪除文件中包含軟連接的部分
	filesToBeDelete, err := model.RemoveFilesWithSoftLinks(fs.FileTarget)
	if err != nil {
		return ErrDBListObjects.WithError(err)
	}

	// 根據儲存策略將文件分組
	policyGroup := fs.GroupFileByPolicy(ctx, filesToBeDelete)

	// 按照儲存策略分組刪除物件
	failed := fs.deleteGroupedFile(ctx, policyGroup)

	// 整理刪除結果
	for i := 0; i < len(fs.FileTarget); i++ {
		if !util.ContainsString(failed[fs.FileTarget[i].PolicyID], fs.FileTarget[i].SourceName) {
			// 已成功刪除的文件
			deletedFileIDs = append(deletedFileIDs, fs.FileTarget[i].ID)
			deletedStorage[fs.FileTarget[i].ID] = fs.FileTarget[i].Size
		}
		// 全部文件
		totalStorage[fs.FileTarget[i].ID] = fs.FileTarget[i].Size
		allFileIDs = append(allFileIDs, fs.FileTarget[i].ID)
	}

	// 如果強制刪除，則將全部文件視為刪除成功
	if force {
		deletedFileIDs = allFileIDs
		deletedStorage = totalStorage
	}

	// 刪除文件記錄
	err = model.DeleteFileByIDs(deletedFileIDs)
	if err != nil {
		return ErrDBDeleteObjects.WithError(err)
	}

	// 刪除文件記錄對應的分享記錄
	model.DeleteShareBySourceIDs(deletedFileIDs, false)

	// 歸還容量
	var total uint64
	for _, value := range deletedStorage {
		total += value
	}
	fs.User.DeductionStorage(total)

	// 如果文件全部刪除成功，繼續刪除目錄
	if len(deletedFileIDs) == len(allFileIDs) {
		var allFolderIDs = make([]uint, 0, len(fs.DirTarget))
		for _, value := range fs.DirTarget {
			allFolderIDs = append(allFolderIDs, value.ID)
		}
		err = model.DeleteFolderByIDs(allFolderIDs)
		if err != nil {
			return ErrDBDeleteObjects.WithError(err)
		}

		// 刪除目錄記錄對應的分享記錄
		model.DeleteShareBySourceIDs(allFolderIDs, true)
	}

	if notDeleted := len(fs.FileTarget) - len(deletedFileIDs); notDeleted > 0 {
		return serializer.NewError(
			serializer.CodeNotFullySuccess,
			fmt.Sprintf("有 %d 個文件未能成功刪除", notDeleted),
			nil,
		)
	}

	return nil
}

// ListDeleteDirs 遞迴列出要刪除目錄，及目錄下所有文件
func (fs *FileSystem) ListDeleteDirs(ctx context.Context, ids []uint) error {
	// 列出所有遞迴子目錄
	folders, err := model.GetRecursiveChildFolder(ids, fs.User.ID, true)
	if err != nil {
		return ErrDBListObjects.WithError(err)
	}
	fs.SetTargetDir(&folders)

	// 檢索目錄下的子文件
	files, err := model.GetChildFilesOfFolders(&folders)
	if err != nil {
		return ErrDBListObjects.WithError(err)
	}
	fs.SetTargetFile(&files)

	return nil
}

// ListDeleteFiles 根據給定的路徑列出要刪除的文件
func (fs *FileSystem) ListDeleteFiles(ctx context.Context, ids []uint) error {
	files, err := model.GetFilesByIDs(ids, fs.User.ID)
	if err != nil {
		return ErrDBListObjects.WithError(err)
	}
	fs.SetTargetFile(&files)
	return nil
}

// List 列出路徑下的內容,
// pathProcessor為最終物件路徑的處理鉤子。
// 有些情況下（如在分享頁面列物件）時，
// 路徑需要截取掉被分享目錄路徑之前的部分。
func (fs *FileSystem) List(ctx context.Context, dirPath string, pathProcessor func(string) string) ([]Object, error) {
	// 獲取父目錄
	isExist, folder := fs.IsPathExist(dirPath)
	if !isExist {
		return nil, ErrPathNotExist
	}
	fs.SetTargetDir(&[]model.Folder{*folder})

	var parentPath = path.Join(folder.Position, folder.Name)
	var childFolders []model.Folder
	var childFiles []model.File

	// 獲取子目錄
	childFolders, _ = folder.GetChildFolder()

	// 獲取子文件
	childFiles, _ = folder.GetChildFiles()

	return fs.listObjects(ctx, parentPath, childFiles, childFolders, pathProcessor), nil
}

// ListPhysical 列出儲存策略中的外部目錄
// TODO:測試
func (fs *FileSystem) ListPhysical(ctx context.Context, dirPath string) ([]Object, error) {
	if err := fs.DispatchHandler(); fs.Policy == nil || err != nil {
		return nil, ErrUnknownPolicyType
	}

	// 儲存策略不支援列取時，返回空結果
	if !fs.Policy.CanStructureBeListed() {
		return nil, nil
	}

	// 列取路徑
	objects, err := fs.Handler.List(ctx, dirPath, false)
	if err != nil {
		return nil, err
	}

	var (
		folders []model.Folder
	)
	for _, object := range objects {
		if object.IsDir {
			folders = append(folders, model.Folder{
				Name: object.Name,
			})
		}
	}

	return fs.listObjects(ctx, dirPath, nil, folders, nil), nil
}

func (fs *FileSystem) listObjects(ctx context.Context, parent string, files []model.File, folders []model.Folder, pathProcessor func(string) string) []Object {
	// 分享文件的ID
	shareKey := ""
	if key, ok := ctx.Value(fsctx.ShareKeyCtx).(string); ok {
		shareKey = key
	}

	// 匯總處理結果
	objects := make([]Object, 0, len(files)+len(folders))

	// 所有物件的父目錄
	var processedPath string

	for _, subFolder := range folders {
		// 路徑處理鉤子，
		// 所有對像父目錄都是一樣的，所以只處理一次
		if processedPath == "" {
			if pathProcessor != nil {
				processedPath = pathProcessor(parent)
			} else {
				processedPath = parent
			}
		}

		objects = append(objects, Object{
			ID:   hashid.HashID(subFolder.ID, hashid.FolderID),
			Name: subFolder.Name,
			Path: processedPath,
			Pic:  "",
			Size: 0,
			Type: "dir",
			Date: subFolder.CreatedAt,
		})
	}

	for _, file := range files {
		if processedPath == "" {
			if pathProcessor != nil {
				processedPath = pathProcessor(parent)
			} else {
				processedPath = parent
			}
		}

		newFile := Object{
			ID:   hashid.HashID(file.ID, hashid.FileID),
			Name: file.Name,
			Path: processedPath,
			Pic:  file.PicInfo,
			Size: file.Size,
			Type: "file",
			Date: file.CreatedAt,
		}
		if shareKey != "" {
			newFile.Key = shareKey
		}
		objects = append(objects, newFile)
	}

	return objects
}

// CreateDirectory 根據給定的完整建立目錄，支援遞迴建立
func (fs *FileSystem) CreateDirectory(ctx context.Context, fullPath string) (*model.Folder, error) {
	if fullPath == "/" || fullPath == "." || fullPath == "" {
		return nil, ErrRootProtected
	}

	// 獲取要建立目錄的父路徑和目錄名
	fullPath = path.Clean(fullPath)
	base := path.Dir(fullPath)
	dir := path.Base(fullPath)

	// 去掉結尾空格
	dir = strings.TrimRight(dir, " ")

	// 檢查目錄名是否合法
	if !fs.ValidateLegalName(ctx, dir) {
		return nil, ErrIllegalObjectName
	}

	// 父目錄是否存在
	isExist, parent := fs.IsPathExist(base)
	if !isExist {
		// 遞迴建立父目錄
		if _, ok := ctx.Value(fsctx.IgnoreDirectoryConflictCtx).(bool); !ok {
			ctx = context.WithValue(ctx, fsctx.IgnoreDirectoryConflictCtx, true)
		}
		newParent, err := fs.CreateDirectory(ctx, base)
		if err != nil {
			return nil, err
		}
		parent = newParent
	}

	// 是否有同名文件
	if ok, _ := fs.IsChildFileExist(parent, dir); ok {
		return nil, ErrFileExisted
	}

	// 建立目錄
	newFolder := model.Folder{
		Name:     dir,
		ParentID: &parent.ID,
		OwnerID:  fs.User.ID,
	}
	_, err := newFolder.Create()

	if err != nil {
		if _, ok := ctx.Value(fsctx.IgnoreDirectoryConflictCtx).(bool); !ok {
			return nil, ErrFolderExisted
		}

	}
	return &newFolder, nil
}

// SaveTo 將別人分享的文件轉存到目標路徑下
func (fs *FileSystem) SaveTo(ctx context.Context, path string) error {
	// 獲取父目錄
	isExist, folder := fs.IsPathExist(path)
	if !isExist {
		return ErrPathNotExist
	}

	var (
		totalSize uint64
		err       error
	)

	if len(fs.DirTarget) > 0 {
		totalSize, err = fs.DirTarget[0].CopyFolderTo(fs.DirTarget[0].ID, folder)
	} else {
		parent := model.Folder{
			OwnerID: fs.FileTarget[0].UserID,
		}
		parent.ID = fs.FileTarget[0].FolderID
		totalSize, err = parent.MoveOrCopyFileTo([]uint{fs.FileTarget[0].ID}, folder, true)
	}

	// 扣除使用者容量
	fs.User.IncreaseStorageWithoutCheck(totalSize)
	if err != nil {
		return ErrFileExisted.WithError(err)
	}

	return nil
}
