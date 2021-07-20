package local

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"

	model "github.com/cloudreve/Cloudreve/v3/models"
	"github.com/cloudreve/Cloudreve/v3/pkg/auth"
	"github.com/cloudreve/Cloudreve/v3/pkg/cache"
	"github.com/cloudreve/Cloudreve/v3/pkg/conf"
	"github.com/cloudreve/Cloudreve/v3/pkg/filesystem/fsctx"
	"github.com/cloudreve/Cloudreve/v3/pkg/filesystem/response"
	"github.com/cloudreve/Cloudreve/v3/pkg/serializer"
	"github.com/cloudreve/Cloudreve/v3/pkg/util"
)

// Driver 本機策略適配器
type Driver struct {
	Policy *model.Policy
}

// List 遞迴列取給定物理路徑下所有文件
func (handler Driver) List(ctx context.Context, path string, recursive bool) ([]response.Object, error) {
	var res []response.Object

	// 取得起始路徑
	root := util.RelativePath(filepath.FromSlash(path))

	// 開始遍歷路徑下的文件、目錄
	err := filepath.Walk(root,
		func(path string, info os.FileInfo, err error) error {
			// 跳過根目錄
			if path == root {
				return nil
			}

			if err != nil {
				util.Log().Warning("無法遍歷目錄 %s, %s", path, err)
				return filepath.SkipDir
			}

			// 將遍歷物件的絕對路徑轉換為相對路徑
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}

			res = append(res, response.Object{
				Name:         info.Name(),
				RelativePath: filepath.ToSlash(rel),
				Source:       path,
				Size:         uint64(info.Size()),
				IsDir:        info.IsDir(),
				LastModify:   info.ModTime(),
			})

			// 如果非遞迴，則不步入目錄
			if !recursive && info.IsDir() {
				return filepath.SkipDir
			}

			return nil
		})

	return res, err
}

// Get 獲取文件內容
func (handler Driver) Get(ctx context.Context, path string) (response.RSCloser, error) {
	// 打開文件
	file, err := os.Open(util.RelativePath(path))
	if err != nil {
		util.Log().Debug("無法打開文件：%s", err)
		return nil, err
	}

	// 開啟一個協程，用於請求結束後關閉reader
	// go closeReader(ctx, file)

	return file, nil
}

// closeReader 用於在請求結束後關閉reader
// TODO 讓業務程式碼自己關閉
func closeReader(ctx context.Context, closer io.Closer) {
	select {
	case <-ctx.Done():
		_ = closer.Close()

	}
}

// Put 將文件流儲存到指定目錄
func (handler Driver) Put(ctx context.Context, file io.ReadCloser, dst string, size uint64) error {
	defer file.Close()
	dst = util.RelativePath(filepath.FromSlash(dst))

	// 如果禁止了 Overwrite，則檢查是否有重名衝突
	if ctx.Value(fsctx.DisableOverwrite) != nil {
		if util.Exists(dst) {
			util.Log().Warning("物理同名文件已存在或不可用: %s", dst)
			return errors.New("物理同名文件已存在或不可用")
		}
	}

	// 如果目標目錄不存在，建立
	basePath := filepath.Dir(dst)
	if !util.Exists(basePath) {
		err := os.MkdirAll(basePath, 0744)
		if err != nil {
			util.Log().Warning("無法建立目錄，%s", err)
			return err
		}
	}

	// 建立目標文件
	out, err := os.Create(dst)
	if err != nil {
		util.Log().Warning("無法建立文件，%s", err)
		return err
	}
	defer out.Close()

	// 寫入檔案內容
	_, err = io.Copy(out, file)
	return err
}

// Delete 刪除一個或多個文件，
// 返回未刪除的文件，及遇到的最後一個錯誤
func (handler Driver) Delete(ctx context.Context, files []string) ([]string, error) {
	deleteFailed := make([]string, 0, len(files))
	var retErr error

	for _, value := range files {
		filePath := util.RelativePath(filepath.FromSlash(value))
		if util.Exists(filePath) {
			err := os.Remove(filePath)
			if err != nil {
				util.Log().Warning("無法刪除文件，%s", err)
				retErr = err
				deleteFailed = append(deleteFailed, value)
			}
		}

		// 嘗試刪除文件的縮圖（如果有）
		_ = os.Remove(util.RelativePath(value + conf.ThumbConfig.FileSuffix))
	}

	return deleteFailed, retErr
}

// Thumb 獲取文件縮圖
func (handler Driver) Thumb(ctx context.Context, path string) (*response.ContentResponse, error) {
	file, err := handler.Get(ctx, path+conf.ThumbConfig.FileSuffix)
	if err != nil {
		return nil, err
	}

	return &response.ContentResponse{
		Redirect: false,
		Content:  file,
	}, nil
}

// Source 獲取外鏈URL
func (handler Driver) Source(
	ctx context.Context,
	path string,
	baseURL url.URL,
	ttl int64,
	isDownload bool,
	speed int,
) (string, error) {
	file, ok := ctx.Value(fsctx.FileModelCtx).(model.File)
	if !ok {
		return "", errors.New("無法獲取文件記錄上下文")
	}

	// 是否啟用了CDN
	if handler.Policy.BaseURL != "" {
		cdnURL, err := url.Parse(handler.Policy.BaseURL)
		if err != nil {
			return "", err
		}
		baseURL = *cdnURL
	}

	var (
		signedURI *url.URL
		err       error
	)
	if isDownload {
		// 建立下載工作階段，將文件訊息寫入快取
		downloadSessionID := util.RandStringRunes(16)
		err = cache.Set("download_"+downloadSessionID, file, int(ttl))
		if err != nil {
			return "", serializer.NewError(serializer.CodeCacheOperation, "無法建立下載工作階段", err)
		}

		// 簽名生成文件記錄
		signedURI, err = auth.SignURI(
			auth.General,
			fmt.Sprintf("/api/v3/file/download/%s", downloadSessionID),
			ttl,
		)
	} else {
		// 簽名生成文件記錄
		signedURI, err = auth.SignURI(
			auth.General,
			fmt.Sprintf("/api/v3/file/get/%d/%s", file.ID, file.Name),
			ttl,
		)
	}

	if err != nil {
		return "", serializer.NewError(serializer.CodeEncryptError, "無法對URL進行簽名", err)
	}

	finalURL := baseURL.ResolveReference(signedURI).String()
	return finalURL, nil
}

// Token 獲取上傳策略和認證Token，本機策略直接返回空值
func (handler Driver) Token(ctx context.Context, ttl int64, key string) (serializer.UploadCredential, error) {
	return serializer.UploadCredential{}, nil
}
