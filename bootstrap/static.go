package bootstrap

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"path"

	"github.com/cloudreve/Cloudreve/v3/pkg/conf"
	"github.com/cloudreve/Cloudreve/v3/pkg/util"
	_ "github.com/cloudreve/Cloudreve/v3/statik"
	"github.com/gin-contrib/static"
	"github.com/rakyll/statik/fs"
)

const StaticFolder = "statics"

type GinFS struct {
	FS http.FileSystem
}

type staticVersion struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// StaticFS 內建靜態文件資源
var StaticFS static.ServeFileSystem

// Open 打開文件
func (b *GinFS) Open(name string) (http.File, error) {
	return b.FS.Open(name)
}

// Exists 文件是否存在
func (b *GinFS) Exists(prefix string, filepath string) bool {

	if _, err := b.FS.Open(filepath); err != nil {
		return false
	}
	return true

}

// InitStatic 初始化靜態資來源文件
func InitStatic() {
	var err error

	if util.Exists(util.RelativePath(StaticFolder)) {
		util.Log().Info("檢測到 statics 目錄存在，將使用此目錄下的靜態資來源文件")
		StaticFS = static.LocalFile(util.RelativePath("statics"), false)

		// 檢查靜態資源的版本
		f, err := StaticFS.Open("version.json")
		if err != nil {
			util.Log().Warning("靜態資源版本標識文件不存在，請重新構建或刪除 statics 目錄")
			return
		}

		b, err := ioutil.ReadAll(f)
		if err != nil {
			util.Log().Warning("無法讀取靜態資來源文件版本，請重新構建或刪除 statics 目錄")
			return
		}

		var v staticVersion
		if err := json.Unmarshal(b, &v); err != nil {
			util.Log().Warning("無法解析靜態資來源文件版本, %s", err)
			return
		}

		staticName := "cloudreve-frontend"
		if conf.IsPro == "true" {
			staticName += "-pro"
		}

		if v.Name != staticName {
			util.Log().Warning("靜態資源版本不匹配，請重新構建或刪除 statics 目錄")
			return
		}

		if v.Version != conf.RequiredStaticVersion {
			util.Log().Warning("靜態資源版本不匹配 [目前 %s, 需要: %s]，請重新構建或刪除 statics 目錄", v.Version, conf.RequiredStaticVersion)
			return
		}

	} else {
		StaticFS = &GinFS{}
		StaticFS.(*GinFS).FS, err = fs.New()
		if err != nil {
			util.Log().Panic("無法初始化靜態資源, %s", err)
		}
	}

}

// Eject 抽離內建靜態資源
func Eject() {
	staticFS, err := fs.New()
	if err != nil {
		util.Log().Panic("無法初始化靜態資源, %s", err)
	}

	root, err := staticFS.Open("/")
	if err != nil {
		util.Log().Panic("根目錄不存在, %s", err)
	}

	var walk func(relPath string, object http.File)
	walk = func(relPath string, object http.File) {
		stat, err := object.Stat()
		if err != nil {
			util.Log().Error("無法獲取[%s]的訊息, %s, 跳過...", relPath, err)
			return
		}

		if !stat.IsDir() {
			// 寫入檔案
			out, err := util.CreatNestedFile(util.RelativePath(StaticFolder + relPath))
			defer out.Close()

			if err != nil {
				util.Log().Error("無法建立文件[%s], %s, 跳過...", relPath, err)
				return
			}

			util.Log().Info("匯出 [%s]...", relPath)
			if _, err := io.Copy(out, object); err != nil {
				util.Log().Error("無法寫入檔案[%s], %s, 跳過...", relPath, err)
				return
			}
		} else {
			// 列出目錄
			objects, err := object.Readdir(0)
			if err != nil {
				util.Log().Error("無法步入子目錄[%s], %s, 跳過...", relPath, err)
				return
			}

			// 遞迴遍歷子目錄
			for _, newObject := range objects {
				newPath := path.Join(relPath, newObject.Name())
				newRoot, err := staticFS.Open(newPath)
				if err != nil {
					util.Log().Error("無法打開物件[%s], %s, 跳過...", newPath, err)
					continue
				}
				walk(newPath, newRoot)
			}

		}
	}

	util.Log().Info("開始匯出內建靜態資源...")
	walk("/", root)
	util.Log().Info("內建靜態資源匯出完成")
}
