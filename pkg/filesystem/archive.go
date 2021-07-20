package filesystem

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	model "github.com/cloudreve/Cloudreve/v3/models"
	"github.com/cloudreve/Cloudreve/v3/pkg/filesystem/fsctx"
	"github.com/cloudreve/Cloudreve/v3/pkg/util"
	"github.com/gin-gonic/gin"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

/* ===============
     壓縮/解壓縮
   ===============
*/

// Compress 建立給定目錄和文件的壓縮文件
func (fs *FileSystem) Compress(ctx context.Context, folderIDs, fileIDs []uint, isArchive bool) (string, error) {
	// 尋找待壓縮目錄
	folders, err := model.GetFoldersByIDs(folderIDs, fs.User.ID)
	if err != nil && len(folderIDs) != 0 {
		return "", ErrDBListObjects
	}

	// 尋找待壓縮文件
	files, err := model.GetFilesByIDs(fileIDs, fs.User.ID)
	if err != nil && len(fileIDs) != 0 {
		return "", ErrDBListObjects
	}

	// 如果上下文限制了父目錄，則進行檢查
	if parent, ok := ctx.Value(fsctx.LimitParentCtx).(*model.Folder); ok {
		// 檢查目錄
		for _, folder := range folders {
			if *folder.ParentID != parent.ID {
				return "", ErrObjectNotExist
			}
		}

		// 檢查文件
		for _, file := range files {
			if file.FolderID != parent.ID {
				return "", ErrObjectNotExist
			}
		}
	}

	// 嘗試獲取請求上下文，以便於後續檢查使用者取消任務
	reqContext := ctx
	ginCtx, ok := ctx.Value(fsctx.GinCtx).(*gin.Context)
	if ok {
		reqContext = ginCtx.Request.Context()
	}

	// 將頂級待處理物件的路徑設為根路徑
	for i := 0; i < len(folders); i++ {
		folders[i].Position = ""
	}
	for i := 0; i < len(files); i++ {
		files[i].Position = ""
	}

	// 建立臨時壓縮文件
	saveFolder := "archive"
	if !isArchive {
		saveFolder = "compress"
	}
	zipFilePath := filepath.Join(
		util.RelativePath(model.GetSettingByName("temp_path")),
		saveFolder,
		fmt.Sprintf("archive_%d.zip", time.Now().UnixNano()),
	)
	zipFile, err := util.CreatNestedFile(zipFilePath)
	if err != nil {
		util.Log().Warning("%s", err)
		return "", err
	}
	defer zipFile.Close()

	// 建立壓縮文件Writer
	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	ctx = reqContext

	// 壓縮各個目錄及文件
	for i := 0; i < len(folders); i++ {
		select {
		case <-reqContext.Done():
			// 取消壓縮請求
			fs.cancelCompress(ctx, zipWriter, zipFile, zipFilePath)
			return "", ErrClientCanceled
		default:
			fs.doCompress(ctx, nil, &folders[i], zipWriter, isArchive)
		}

	}
	for i := 0; i < len(files); i++ {
		select {
		case <-reqContext.Done():
			// 取消壓縮請求
			fs.cancelCompress(ctx, zipWriter, zipFile, zipFilePath)
			return "", ErrClientCanceled
		default:
			fs.doCompress(ctx, &files[i], nil, zipWriter, isArchive)
		}
	}

	return zipFilePath, nil
}

// cancelCompress 取消壓縮行程
func (fs *FileSystem) cancelCompress(ctx context.Context, zipWriter *zip.Writer, file *os.File, path string) {
	util.Log().Debug("用戶端取消壓縮請求")
	zipWriter.Close()
	file.Close()
	_ = os.Remove(path)
}

func (fs *FileSystem) doCompress(ctx context.Context, file *model.File, folder *model.Folder, zipWriter *zip.Writer, isArchive bool) {
	// 如果物件是文件
	if file != nil {
		// 切換上傳策略
		fs.Policy = file.GetPolicy()
		err := fs.DispatchHandler()
		if err != nil {
			util.Log().Warning("無法壓縮文件%s，%s", file.Name, err)
			return
		}

		// 獲取文件內容
		fileToZip, err := fs.Handler.Get(
			context.WithValue(ctx, fsctx.FileModelCtx, *file),
			file.SourceName,
		)
		if err != nil {
			util.Log().Debug("Open%s，%s", file.Name, err)
			return
		}
		if closer, ok := fileToZip.(io.Closer); ok {
			defer closer.Close()
		}

		// 建立壓縮文件頭
		header := &zip.FileHeader{
			Name:               filepath.FromSlash(path.Join(file.Position, file.Name)),
			Modified:           file.UpdatedAt,
			UncompressedSize64: file.Size,
		}

		// 指定是壓縮還是歸檔
		if isArchive {
			header.Method = zip.Store
		} else {
			header.Method = zip.Deflate
		}

		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			return
		}

		_, err = io.Copy(writer, fileToZip)
	} else if folder != nil {
		// 物件是目錄
		// 獲取子文件
		subFiles, err := folder.GetChildFiles()
		if err == nil && len(subFiles) > 0 {
			for i := 0; i < len(subFiles); i++ {
				fs.doCompress(ctx, &subFiles[i], nil, zipWriter, isArchive)
			}

		}
		// 獲取子目錄，繼續遞迴遍歷
		subFolders, err := folder.GetChildFolder()
		if err == nil && len(subFolders) > 0 {
			for i := 0; i < len(subFolders); i++ {
				fs.doCompress(ctx, nil, &subFolders[i], zipWriter, isArchive)
			}
		}
	}
}

// Decompress 解壓縮給定壓縮文件到dst目錄
func (fs *FileSystem) Decompress(ctx context.Context, src, dst string) error {
	err := fs.ResetFileIfNotExist(ctx, src)
	if err != nil {
		return err
	}

	tempZipFilePath := ""
	defer func() {
		// 結束時刪除臨時壓縮文件
		if tempZipFilePath != "" {
			if err := os.Remove(tempZipFilePath); err != nil {
				util.Log().Warning("無法刪除臨時壓縮文件 %s , %s", tempZipFilePath, err)
			}
		}
	}()

	// 下載壓縮文件到暫存資料夾
	fileStream, err := fs.Handler.Get(ctx, fs.FileTarget[0].SourceName)
	if err != nil {
		return err
	}

	tempZipFilePath = filepath.Join(
		util.RelativePath(model.GetSettingByName("temp_path")),
		"decompress",
		fmt.Sprintf("archive_%d.zip", time.Now().UnixNano()),
	)

	zipFile, err := util.CreatNestedFile(tempZipFilePath)
	if err != nil {
		util.Log().Warning("無法建立臨時壓縮文件 %s , %s", tempZipFilePath, err)
		tempZipFilePath = ""
		return err
	}
	defer zipFile.Close()

	_, err = io.Copy(zipFile, fileStream)
	if err != nil {
		util.Log().Warning("無法寫入臨時壓縮文件 %s , %s", tempZipFilePath, err)
		return err
	}

	zipFile.Close()

	// 解壓縮文件
	r, err := zip.OpenReader(tempZipFilePath)
	if err != nil {
		return err
	}
	defer r.Close()

	// 重設儲存策略
	fs.Policy = &fs.User.Policy
	err = fs.DispatchHandler()
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	parallel := model.GetIntSetting("max_parallel_transfer", 4)
	worker := make(chan int, parallel)
	for i := 0; i < parallel; i++ {
		worker <- i
	}

	for _, f := range r.File {
		fileName := f.Name
		// 處理非UTF-8編碼
		if f.NonUTF8 {
			i := bytes.NewReader([]byte(fileName))
			decoder := transform.NewReader(i, simplifiedchinese.GB18030.NewDecoder())
			content, _ := ioutil.ReadAll(decoder)
			fileName = string(content)
		}

		rawPath := util.FormSlash(fileName)
		savePath := path.Join(dst, rawPath)
		// 路徑是否合法
		if !strings.HasPrefix(savePath, util.FillSlash(path.Clean(dst))) {
			return fmt.Errorf("%s: illegal file path", f.Name)
		}

		// 如果是目錄
		if f.FileInfo().IsDir() {
			fs.CreateDirectory(ctx, savePath)
			continue
		}

		// 上傳文件
		fileStream, err := f.Open()
		if err != nil {
			util.Log().Warning("無法打開壓縮包內文件%s , %s , 跳過", rawPath, err)
			continue
		}

		select {
		case <-worker:
			wg.Add(1)
			go func(fileStream io.ReadCloser, size int64) {
				defer func() {
					worker <- 1
					wg.Done()
					if err := recover(); err != nil {
						util.Log().Warning("上傳壓縮包內文件時出錯")
						fmt.Println(err)
					}
				}()

				err = fs.UploadFromStream(ctx, fileStream, savePath, uint64(size))
				fileStream.Close()
				if err != nil {
					util.Log().Debug("無法上傳壓縮包內的文件%s , %s , 跳過", rawPath, err)
				}
			}(fileStream, f.FileInfo().Size())
		}

	}
	wg.Wait()
	return nil

}
