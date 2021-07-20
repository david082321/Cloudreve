package hashid

import (
	"errors"

	"github.com/cloudreve/Cloudreve/v3/pkg/conf"
	"github.com/speps/go-hashids"
)

// ID類型
const (
	ShareID  = iota // 分享
	UserID          // 使用者
	FileID          // 文件ID
	FolderID        // 目錄ID
	TagID           // 標籤ID
	PolicyID        // 儲存策略ID
)

var (
	// ErrTypeNotMatch ID類型不匹配
	ErrTypeNotMatch = errors.New("ID類型不匹配")
)

// HashEncode 對給定資料計算HashID
func HashEncode(v []int) (string, error) {
	hd := hashids.NewData()
	hd.Salt = conf.SystemConfig.HashIDSalt

	h, err := hashids.NewWithData(hd)
	if err != nil {
		return "", err
	}

	id, err := h.Encode(v)
	if err != nil {
		return "", err
	}
	return id, nil
}

// HashDecode 對給定資料計算原始資料
func HashDecode(raw string) ([]int, error) {
	hd := hashids.NewData()
	hd.Salt = conf.SystemConfig.HashIDSalt

	h, err := hashids.NewWithData(hd)
	if err != nil {
		return []int{}, err
	}

	return h.DecodeWithError(raw)

}

// HashID 計算資料庫內主鍵對應的HashID
func HashID(id uint, t int) string {
	v, _ := HashEncode([]int{int(id), t})
	return v
}

// DecodeHashID 計算HashID對應的資料庫ID
func DecodeHashID(id string, t int) (uint, error) {
	v, _ := HashDecode(id)
	if len(v) != 2 || v[1] != t {
		return 0, ErrTypeNotMatch
	}
	return uint(v[0]), nil
}
