package serializer

import (
	"encoding/base64"
	"encoding/gob"
	"encoding/json"
)

// UploadPolicy slave模式下傳遞的上傳策略
type UploadPolicy struct {
	SavePath         string   `json:"save_path"`
	FileName         string   `json:"file_name"`
	AutoRename       bool     `json:"auto_rename"`
	MaxSize          uint64   `json:"max_size"`
	AllowedExtension []string `json:"allowed_extension"`
	CallbackURL      string   `json:"callback_url"`
}

// UploadCredential 返回給用戶端的上傳憑證
type UploadCredential struct {
	Token     string `json:"token"`
	Policy    string `json:"policy"`
	Path      string `json:"path"` // 儲存路徑
	AccessKey string `json:"ak"`
	KeyTime   string `json:"key_time,omitempty"` // COS用有效期
	Callback  string `json:"callback,omitempty"` // 回調地址
	Key       string `json:"key,omitempty"`      // 文件標識符，通常為回調key
}

// UploadSession 上傳工作階段
type UploadSession struct {
	Key         string
	UID         uint
	PolicyID    uint
	VirtualPath string
	Name        string
	Size        uint64
	SavePath    string
}

// UploadCallback 上傳回調正文
type UploadCallback struct {
	Name       string `json:"name"`
	SourceName string `json:"source_name"`
	PicInfo    string `json:"pic_info"`
	Size       uint64 `json:"size"`
}

// GeneralUploadCallbackFailed 儲存策略上傳回調失敗響應
type GeneralUploadCallbackFailed struct {
	Error string `json:"error"`
}

func init() {
	gob.Register(UploadSession{})
}

// DecodeUploadPolicy 反序列化Header中攜帶的上傳策略
func DecodeUploadPolicy(raw string) (*UploadPolicy, error) {
	var res UploadPolicy

	rawJSON, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(rawJSON, &res)
	if err != nil {
		return nil, err
	}

	return &res, err
}

// EncodeUploadPolicy 序列化Header中攜帶的上傳策略
func (policy *UploadPolicy) EncodeUploadPolicy() (string, error) {
	jsonRes, err := json.Marshal(policy)
	if err != nil {
		return "", err
	}

	res := base64.StdEncoding.EncodeToString(jsonRes)
	return res, nil

}
