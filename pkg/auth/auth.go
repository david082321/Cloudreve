package auth

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	model "github.com/cloudreve/Cloudreve/v3/models"
	"github.com/cloudreve/Cloudreve/v3/pkg/conf"
	"github.com/cloudreve/Cloudreve/v3/pkg/serializer"
	"github.com/cloudreve/Cloudreve/v3/pkg/util"
)

var (
	ErrAuthFailed = serializer.NewError(serializer.CodeNoPermissionErr, "鑒權失敗", nil)
	ErrExpired    = serializer.NewError(serializer.CodeSignExpired, "簽名已過期", nil)
)

// General 通用的認證介面
var General Auth

// Auth 鑒權認證
type Auth interface {
	// 對給定Body進行簽名,expires為0表示永不過期
	Sign(body string, expires int64) string
	// 對給定Body和Sign進行檢查
	Check(body string, sign string) error
}

// SignRequest 對PUT\POST等複雜HTTP請求簽名，如果請求Header中
// 包含 X-Policy， 則此請求會被認定為上傳請求，只會對URI部分和
// Policy部分進行簽名。其他請求則會對URI和Body部分進行簽名。
func SignRequest(instance Auth, r *http.Request, expires int64) *http.Request {
	// 處理有效期
	if expires > 0 {
		expires += time.Now().Unix()
	}

	// 生成簽名
	sign := instance.Sign(getSignContent(r), expires)

	// 將簽名加到請求Header中
	r.Header["Authorization"] = []string{"Bearer " + sign}
	return r
}

// CheckRequest 對複雜請求進行簽名驗證
func CheckRequest(instance Auth, r *http.Request) error {
	var (
		sign []string
		ok   bool
	)
	if sign, ok = r.Header["Authorization"]; !ok || len(sign) == 0 {
		return ErrAuthFailed
	}
	sign[0] = strings.TrimPrefix(sign[0], "Bearer ")

	return instance.Check(getSignContent(r), sign[0])
}

// getSignContent 根據請求Header中是否包含X-Policy判斷是否為上傳請求，
// 返回待簽名/驗證的字串
func getSignContent(r *http.Request) (rawSignString string) {
	if policy, ok := r.Header["X-Policy"]; ok {
		rawSignString = serializer.NewRequestSignString(r.URL.Path, policy[0], "")
	} else {
		var body = []byte{}
		if r.Body != nil {
			body, _ = ioutil.ReadAll(r.Body)
			_ = r.Body.Close()
			r.Body = ioutil.NopCloser(bytes.NewReader(body))
		}
		rawSignString = serializer.NewRequestSignString(r.URL.Path, "", string(body))
	}
	return rawSignString
}

// SignURI 對URI進行簽名,簽名只針對Path部分，query部分不做驗證
func SignURI(instance Auth, uri string, expires int64) (*url.URL, error) {
	// 處理有效期
	if expires != 0 {
		expires += time.Now().Unix()
	}

	base, err := url.Parse(uri)
	if err != nil {
		return nil, err
	}

	// 生成簽名
	sign := instance.Sign(base.Path, expires)

	// 將簽名加到URI中
	queries := base.Query()
	queries.Set("sign", sign)
	base.RawQuery = queries.Encode()

	return base, nil
}

// CheckURI 對URI進行鑒權
func CheckURI(instance Auth, url *url.URL) error {
	//獲取待驗證的簽名正文
	queries := url.Query()
	sign := queries.Get("sign")
	queries.Del("sign")
	url.RawQuery = queries.Encode()

	return instance.Check(url.Path, sign)
}

// Init 初始化通用鑒權器
func Init() {
	var secretKey string
	if conf.SystemConfig.Mode == "master" {
		secretKey = model.GetSettingByName("secret_key")
	} else {
		secretKey = conf.SlaveConfig.Secret
		if secretKey == "" {
			util.Log().Panic("未指定 SlaveSecret，請前往配置檔案中指定")
		}
	}
	General = HMACAuth{
		SecretKey: []byte(secretKey),
	}
}
