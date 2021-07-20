package share

import (
	"context"
	"fmt"
	"net/http"
	"path"

	model "github.com/cloudreve/Cloudreve/v3/models"
	"github.com/cloudreve/Cloudreve/v3/pkg/filesystem"
	"github.com/cloudreve/Cloudreve/v3/pkg/filesystem/fsctx"
	"github.com/cloudreve/Cloudreve/v3/pkg/hashid"
	"github.com/cloudreve/Cloudreve/v3/pkg/serializer"
	"github.com/cloudreve/Cloudreve/v3/pkg/util"
	"github.com/cloudreve/Cloudreve/v3/service/explorer"
	"github.com/gin-gonic/gin"
)

// ShareUserGetService 獲取使用者的分享服務
type ShareUserGetService struct {
	Type string `form:"type" binding:"required,eq=hot|eq=default"`
	Page uint   `form:"page" binding:"required,min=1"`
}

// ShareGetService 獲取分享服務
type ShareGetService struct {
	Password string `form:"password" binding:"max=255"`
}

// Service 對分享進行操作的服務，
// path 為可選文件完整路徑，在目錄分享下有效
type Service struct {
	Path string `form:"path" uri:"path" binding:"max=65535"`
}

// ArchiveService 分享歸檔下載服務
type ArchiveService struct {
	Path  string   `json:"path" binding:"required,max=65535"`
	Items []string `json:"items"`
	Dirs  []string `json:"dirs"`
}

// ShareListService 列出分享
type ShareListService struct {
	Page     uint   `form:"page" binding:"required,min=1"`
	OrderBy  string `form:"order_by" binding:"required,eq=created_at|eq=downloads|eq=views"`
	Order    string `form:"order" binding:"required,eq=DESC|eq=ASC"`
	Keywords string `form:"keywords"`
}

// Get 獲取給定使用者的分享
func (service *ShareUserGetService) Get(c *gin.Context) serializer.Response {
	// 取得使用者
	userID, _ := c.Get("object_id")
	user, err := model.GetActiveUserByID(userID.(uint))
	if err != nil || user.OptionsSerialized.ProfileOff {
		return serializer.Err(serializer.CodeNotFound, "使用者不存在", err)
	}

	// 列出分享
	hotNum := model.GetIntSetting("hot_share_num", 10)
	if service.Type == "default" {
		hotNum = 10
	}
	orderBy := "created_at desc"
	if service.Type == "hot" {
		orderBy = "views desc"
	}
	shares, total := model.ListShares(user.ID, int(service.Page), hotNum, orderBy, true)
	// 列出分享對應的文件
	for i := 0; i < len(shares); i++ {
		shares[i].Source()
	}

	res := serializer.BuildShareList(shares, total)
	res.Data.(map[string]interface{})["user"] = struct {
		ID    string `json:"id"`
		Nick  string `json:"nick"`
		Group string `json:"group"`
		Date  string `json:"date"`
	}{
		hashid.HashID(user.ID, hashid.UserID),
		user.Nick,
		user.Group.Name,
		user.CreatedAt.Format("2006-01-02 15:04:05"),
	}

	return res
}

// Search 搜尋公共分享
func (service *ShareListService) Search(c *gin.Context) serializer.Response {
	// 列出分享
	shares, total := model.SearchShares(int(service.Page), 18, service.OrderBy+" "+
		service.Order, service.Keywords)
	// 列出分享對應的文件
	for i := 0; i < len(shares); i++ {
		shares[i].Source()
	}

	return serializer.BuildShareList(shares, total)
}

// List 列出使用者分享
func (service *ShareListService) List(c *gin.Context, user *model.User) serializer.Response {
	// 列出分享
	shares, total := model.ListShares(user.ID, int(service.Page), 18, service.OrderBy+" "+
		service.Order, false)
	// 列出分享對應的文件
	for i := 0; i < len(shares); i++ {
		shares[i].Source()
	}

	return serializer.BuildShareList(shares, total)
}

// Get 獲取分享內容
func (service *ShareGetService) Get(c *gin.Context) serializer.Response {
	shareCtx, _ := c.Get("share")
	share := shareCtx.(*model.Share)

	// 是否已解鎖
	unlocked := true
	if share.Password != "" {
		sessionKey := fmt.Sprintf("share_unlock_%d", share.ID)
		unlocked = util.GetSession(c, sessionKey) != nil
		if !unlocked && service.Password != "" {
			// 如果未解鎖，且指定了密碼，則嘗試解鎖
			if service.Password == share.Password {
				unlocked = true
				util.SetSession(c, map[string]interface{}{sessionKey: true})
			}
		}
	}

	if unlocked {
		share.Viewed()
	}

	return serializer.Response{
		Code: 0,
		Data: serializer.BuildShareResponse(share, unlocked),
	}
}

// CreateDownloadSession 建立下載工作階段
func (service *Service) CreateDownloadSession(c *gin.Context) serializer.Response {
	shareCtx, _ := c.Get("share")
	share := shareCtx.(*model.Share)
	userCtx, _ := c.Get("user")
	user := userCtx.(*model.User)

	// 建立文件系統
	fs, err := filesystem.NewFileSystem(user)
	if err != nil {
		return serializer.Err(serializer.CodePolicyNotAllowed, err.Error(), err)
	}
	defer fs.Recycle()

	// 重設文件系統處理目標為來源文件
	err = fs.SetTargetByInterface(share.Source())
	if err != nil {
		return serializer.Err(serializer.CodePolicyNotAllowed, "來源文件不存在", err)
	}

	ctx := context.Background()

	// 重設根目錄
	if share.IsDir {
		fs.Root = &fs.DirTarget[0]

		// 找到目標文件
		err = fs.ResetFileIfNotExist(ctx, service.Path)
		if err != nil {
			return serializer.Err(serializer.CodeNotSet, err.Error(), err)
		}
	}

	// 取得下載網址
	downloadURL, err := fs.GetDownloadURL(ctx, 0, "download_timeout")
	if err != nil {
		return serializer.Err(serializer.CodeNotSet, err.Error(), err)
	}

	return serializer.Response{
		Code: 0,
		Data: downloadURL,
	}
}

// PreviewContent 預覽文件，需要登入工作階段, isText - 是否為文字文件，文字文件會
// 強制經由服務端中轉
func (service *Service) PreviewContent(ctx context.Context, c *gin.Context, isText bool) serializer.Response {
	shareCtx, _ := c.Get("share")
	share := shareCtx.(*model.Share)

	// 用於調下層service
	if share.IsDir {
		ctx = context.WithValue(ctx, fsctx.FolderModelCtx, share.Source())
		ctx = context.WithValue(ctx, fsctx.PathCtx, service.Path)
	} else {
		ctx = context.WithValue(ctx, fsctx.FileModelCtx, share.Source())
	}
	subService := explorer.FileIDService{}

	return subService.PreviewContent(ctx, c, isText)
}

// CreateDocPreviewSession 建立Office預覽工作階段，返回預覽地址
func (service *Service) CreateDocPreviewSession(c *gin.Context) serializer.Response {
	shareCtx, _ := c.Get("share")
	share := shareCtx.(*model.Share)

	// 用於調下層service
	ctx := context.Background()
	if share.IsDir {
		ctx = context.WithValue(ctx, fsctx.FolderModelCtx, share.Source())
		ctx = context.WithValue(ctx, fsctx.PathCtx, service.Path)
	} else {
		ctx = context.WithValue(ctx, fsctx.FileModelCtx, share.Source())
	}
	subService := explorer.FileIDService{}

	return subService.CreateDocPreviewSession(ctx, c)
}

// List 列出分享的目錄下的物件
func (service *Service) List(c *gin.Context) serializer.Response {
	shareCtx, _ := c.Get("share")
	share := shareCtx.(*model.Share)

	if !share.IsDir {
		return serializer.ParamErr("此分享無法列目錄", nil)
	}

	if !path.IsAbs(service.Path) {
		return serializer.ParamErr("路徑無效", nil)
	}

	// 建立文件系統
	fs, err := filesystem.NewFileSystem(share.Creator())
	if err != nil {
		return serializer.Err(serializer.CodePolicyNotAllowed, err.Error(), err)
	}
	defer fs.Recycle()

	// 上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 重設根目錄
	fs.Root = share.Source().(*model.Folder)
	fs.Root.Name = "/"

	// 分享Key上下文
	ctx = context.WithValue(ctx, fsctx.ShareKeyCtx, hashid.HashID(share.ID, hashid.ShareID))

	// 獲取子項目
	objects, err := fs.List(ctx, service.Path, nil)
	if err != nil {
		return serializer.Err(serializer.CodeCreateFolderFailed, err.Error(), err)
	}

	return serializer.Response{
		Code: 0,
		Data: map[string]interface{}{
			"parent":  "0000",
			"objects": objects,
		},
	}
}

// Thumb 獲取被分享文件的縮圖
func (service *Service) Thumb(c *gin.Context) serializer.Response {
	shareCtx, _ := c.Get("share")
	share := shareCtx.(*model.Share)

	if !share.IsDir {
		return serializer.ParamErr("此分享無縮圖", nil)
	}

	// 建立文件系統
	fs, err := filesystem.NewFileSystem(share.Creator())
	if err != nil {
		return serializer.Err(serializer.CodePolicyNotAllowed, err.Error(), err)
	}
	defer fs.Recycle()

	// 重設根目錄
	fs.Root = share.Source().(*model.Folder)

	// 找到縮圖的父目錄
	exist, parent := fs.IsPathExist(service.Path)
	if !exist {
		return serializer.Err(serializer.CodeNotFound, "路徑不存在", nil)
	}

	ctx := context.WithValue(context.Background(), fsctx.LimitParentCtx, parent)

	// 獲取文件ID
	fileID, err := hashid.DecodeHashID(c.Param("file"), hashid.FileID)
	if err != nil {
		return serializer.ParamErr("無法解析文件ID", err)
	}

	// 獲取縮圖
	resp, err := fs.GetThumb(ctx, uint(fileID))
	if err != nil {
		return serializer.Err(serializer.CodeNotSet, "無法獲取縮圖", err)
	}

	if resp.Redirect {
		c.Header("Cache-Control", fmt.Sprintf("max-age=%d", resp.MaxAge))
		c.Redirect(http.StatusMovedPermanently, resp.URL)
		return serializer.Response{Code: -1}
	}

	defer resp.Content.Close()
	http.ServeContent(c.Writer, c.Request, "thumb.png", fs.FileTarget[0].UpdatedAt, resp.Content)

	return serializer.Response{Code: -1}

}

// Archive 建立批次下載歸檔
func (service *ArchiveService) Archive(c *gin.Context) serializer.Response {
	shareCtx, _ := c.Get("share")
	share := shareCtx.(*model.Share)
	userCtx, _ := c.Get("user")
	user := userCtx.(*model.User)

	// 是否有權限
	if !user.Group.OptionsSerialized.ArchiveDownload {
		return serializer.Err(serializer.CodeNoPermissionErr, "您的使用者群組無權進行此操作", nil)
	}

	if !share.IsDir {
		return serializer.ParamErr("此分享無法進行打包", nil)
	}

	// 建立文件系統
	fs, err := filesystem.NewFileSystem(user)
	if err != nil {
		return serializer.Err(serializer.CodePolicyNotAllowed, err.Error(), err)
	}
	defer fs.Recycle()

	// 重設根目錄
	fs.Root = share.Source().(*model.Folder)

	// 找到要打包文件的父目錄
	exist, parent := fs.IsPathExist(service.Path)
	if !exist {
		return serializer.Err(serializer.CodeNotFound, "路徑不存在", nil)
	}

	// 限制操作範圍為父目錄下
	ctx := context.WithValue(context.Background(), fsctx.LimitParentCtx, parent)

	// 用於調下層service
	tempUser := share.Creator()
	tempUser.Group.OptionsSerialized.ArchiveDownload = true
	c.Set("user", tempUser)

	subService := explorer.ItemIDService{
		Dirs:  service.Dirs,
		Items: service.Items,
	}

	return subService.Archive(ctx, c)
}
