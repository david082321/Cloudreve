package filesystem

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"sync"

	model "github.com/cloudreve/Cloudreve/v3/models"
	"github.com/cloudreve/Cloudreve/v3/pkg/auth"
	"github.com/cloudreve/Cloudreve/v3/pkg/conf"
	"github.com/cloudreve/Cloudreve/v3/pkg/filesystem/driver/cos"
	"github.com/cloudreve/Cloudreve/v3/pkg/filesystem/driver/local"
	"github.com/cloudreve/Cloudreve/v3/pkg/filesystem/driver/onedrive"
	"github.com/cloudreve/Cloudreve/v3/pkg/filesystem/driver/oss"
	"github.com/cloudreve/Cloudreve/v3/pkg/filesystem/driver/qiniu"
	"github.com/cloudreve/Cloudreve/v3/pkg/filesystem/driver/remote"
	"github.com/cloudreve/Cloudreve/v3/pkg/filesystem/driver/s3"
	"github.com/cloudreve/Cloudreve/v3/pkg/filesystem/driver/upyun"
	"github.com/cloudreve/Cloudreve/v3/pkg/filesystem/response"
	"github.com/cloudreve/Cloudreve/v3/pkg/request"
	"github.com/cloudreve/Cloudreve/v3/pkg/serializer"
	"github.com/gin-gonic/gin"
	cossdk "github.com/tencentyun/cos-go-sdk-v5"
)

// FSPool 文件系統資源池
var FSPool = sync.Pool{
	New: func() interface{} {
		return &FileSystem{}
	},
}

// FileHeader 上傳來的文件資料處理器
type FileHeader interface {
	io.Reader
	io.Closer
	GetSize() uint64
	GetMIMEType() string
	GetFileName() string
	GetVirtualPath() string
}

// Handler 儲存策略適配器
type Handler interface {
	// 上傳文件, dst為文件儲存路徑，size 為檔案大小。上下文關閉
	// 時，應取消上傳並清理暫存檔
	Put(ctx context.Context, file io.ReadCloser, dst string, size uint64) error

	// 刪除一個或多個給定路徑的文件，返回刪除失敗的文件路徑列表及錯誤
	Delete(ctx context.Context, files []string) ([]string, error)

	// 獲取文件內容
	Get(ctx context.Context, path string) (response.RSCloser, error)

	// 獲取縮圖，可直接在ContentResponse中返回文件資料流，也可指
	// 定為重定向
	Thumb(ctx context.Context, path string) (*response.ContentResponse, error)

	// 獲取外鏈/下載網址，
	// url - 站點本身地址,
	// isDownload - 是否直接下載
	Source(ctx context.Context, path string, url url.URL, ttl int64, isDownload bool, speed int) (string, error)

	// Token 獲取有效期為ttl的上傳憑證和簽名，同時回調工作階段有效期為sessionTTL
	Token(ctx context.Context, ttl int64, callbackKey string) (serializer.UploadCredential, error)

	// List 遞迴列取遠端端path路徑下文件、目錄，不包含path本身，
	// 返回的物件路徑以path作為起始根目錄.
	// recursive - 是否遞迴列出
	List(ctx context.Context, path string, recursive bool) ([]response.Object, error)
}

// FileSystem 管理文件的文件系統
type FileSystem struct {
	// 文件系統所有者
	User *model.User
	// 操作文件使用的儲存策略
	Policy *model.Policy
	// 目前正在處理的文件物件
	FileTarget []model.File
	// 目前正在處理的目錄物件
	DirTarget []model.Folder
	// 相對根目錄
	Root *model.Folder
	// 互斥鎖
	Lock sync.Mutex

	/*
	   鉤子函數
	*/
	Hooks map[string][]Hook

	/*
	   文件系統處理適配器
	*/
	Handler Handler

	// 回收鎖
	recycleLock sync.Mutex
}

// getEmptyFS 從pool中獲取新的FileSystem
func getEmptyFS() *FileSystem {
	fs := FSPool.Get().(*FileSystem)
	return fs
}

// Recycle 回收FileSystem資源
func (fs *FileSystem) Recycle() {
	fs.recycleLock.Lock()
	fs.reset()
	FSPool.Put(fs)
}

// reset 重設文件系統，以便回收使用
func (fs *FileSystem) reset() {
	fs.User = nil
	fs.CleanTargets()
	fs.Policy = nil
	fs.Hooks = nil
	fs.Handler = nil
	fs.Root = nil
	fs.Lock = sync.Mutex{}
	fs.recycleLock = sync.Mutex{}
}

// NewFileSystem 初始化一個文件系統
func NewFileSystem(user *model.User) (*FileSystem, error) {
	fs := getEmptyFS()
	fs.User = user
	// 分配儲存策略適配器
	err := fs.DispatchHandler()

	// TODO 分配預設鉤子
	return fs, err
}

// NewAnonymousFileSystem 初始化匿名文件系統
func NewAnonymousFileSystem() (*FileSystem, error) {
	fs := getEmptyFS()
	fs.User = &model.User{}

	// 如果是主機模式下，則為匿名文件系統分配遊客使用者群組
	if conf.SystemConfig.Mode == "master" {
		anonymousGroup, err := model.GetGroupByID(3)
		if err != nil {
			return nil, err
		}
		fs.User.Group = anonymousGroup
	} else {
		// 從機模式下，分配本機策略處理器
		fs.Handler = local.Driver{}
	}

	return fs, nil
}

// DispatchHandler 根據儲存策略分配文件適配器
// TODO 完善測試
func (fs *FileSystem) DispatchHandler() error {
	var policyType string
	var currentPolicy *model.Policy

	if fs.Policy == nil {
		// 如果沒有具體指定，就是用使用者目前儲存策略
		policyType = fs.User.Policy.Type
		currentPolicy = &fs.User.Policy
	} else {
		policyType = fs.Policy.Type
		currentPolicy = fs.Policy
	}

	switch policyType {
	case "mock", "anonymous":
		return nil
	case "local":
		fs.Handler = local.Driver{
			Policy: currentPolicy,
		}
		return nil
	case "remote":
		fs.Handler = remote.Driver{
			Policy:       currentPolicy,
			Client:       request.HTTPClient{},
			AuthInstance: auth.HMACAuth{[]byte(currentPolicy.SecretKey)},
		}
		return nil
	case "qiniu":
		fs.Handler = qiniu.Driver{
			Policy: currentPolicy,
		}
		return nil
	case "oss":
		fs.Handler = oss.Driver{
			Policy:     currentPolicy,
			HTTPClient: request.HTTPClient{},
		}
		return nil
	case "upyun":
		fs.Handler = upyun.Driver{
			Policy: currentPolicy,
		}
		return nil
	case "onedrive":
		client, err := onedrive.NewClient(currentPolicy)
		fs.Handler = onedrive.Driver{
			Policy:     currentPolicy,
			Client:     client,
			HTTPClient: request.HTTPClient{},
		}
		return err
	case "cos":
		u, _ := url.Parse(currentPolicy.Server)
		b := &cossdk.BaseURL{BucketURL: u}
		fs.Handler = cos.Driver{
			Policy: currentPolicy,
			Client: cossdk.NewClient(b, &http.Client{
				Transport: &cossdk.AuthorizationTransport{
					SecretID:  currentPolicy.AccessKey,
					SecretKey: currentPolicy.SecretKey,
				},
			}),
			HTTPClient: request.HTTPClient{},
		}
		return nil
	case "s3":
		fs.Handler = s3.Driver{
			Policy: currentPolicy,
		}
		return nil
	default:
		return ErrUnknownPolicyType
	}
}

// NewFileSystemFromContext 從gin.Context建立文件系統
func NewFileSystemFromContext(c *gin.Context) (*FileSystem, error) {
	user, exist := c.Get("user")
	if !exist {
		return NewAnonymousFileSystem()
	}
	fs, err := NewFileSystem(user.(*model.User))
	return fs, err
}

// NewFileSystemFromCallback 從gin.Context建立回呼叫文件系統
func NewFileSystemFromCallback(c *gin.Context) (*FileSystem, error) {
	fs, err := NewFileSystemFromContext(c)
	if err != nil {
		return nil, err
	}

	// 獲取回調工作階段
	callbackSessionRaw, ok := c.Get("callbackSession")
	if !ok {
		return nil, errors.New("找不到回調工作階段")
	}
	callbackSession := callbackSessionRaw.(*serializer.UploadSession)

	// 重新指向上傳策略
	policy, err := model.GetPolicyByID(callbackSession.PolicyID)
	if err != nil {
		return nil, err
	}
	fs.Policy = &policy
	fs.User.Policy = policy
	err = fs.DispatchHandler()

	return fs, err
}

// SetTargetFile 設定目前處理的目標文件
func (fs *FileSystem) SetTargetFile(files *[]model.File) {
	if len(fs.FileTarget) == 0 {
		fs.FileTarget = *files
	} else {
		fs.FileTarget = append(fs.FileTarget, *files...)
	}

}

// SetTargetDir 設定目前處理的目標目錄
func (fs *FileSystem) SetTargetDir(dirs *[]model.Folder) {
	if len(fs.DirTarget) == 0 {
		fs.DirTarget = *dirs
	} else {
		fs.DirTarget = append(fs.DirTarget, *dirs...)
	}

}

// SetTargetFileByIDs 根據文件ID設定目標文件，忽略使用者ID
func (fs *FileSystem) SetTargetFileByIDs(ids []uint) error {
	files, err := model.GetFilesByIDs(ids, 0)
	if err != nil || len(files) == 0 {
		return ErrFileExisted.WithError(err)
	}
	fs.SetTargetFile(&files)
	return nil
}

// SetTargetByInterface 根據 model.File 或者 model.Folder 設定目標物件
// TODO 測試
func (fs *FileSystem) SetTargetByInterface(target interface{}) error {
	if file, ok := target.(*model.File); ok {
		fs.SetTargetFile(&[]model.File{*file})
		return nil
	}
	if folder, ok := target.(*model.Folder); ok {
		fs.SetTargetDir(&[]model.Folder{*folder})
		return nil
	}

	return ErrObjectNotExist
}

// CleanTargets 清空目標
func (fs *FileSystem) CleanTargets() {
	fs.FileTarget = fs.FileTarget[:0]
	fs.DirTarget = fs.DirTarget[:0]
}
