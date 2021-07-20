package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"io"
	"strconv"
	"strings"
	"time"
)

// HMACAuth HMAC演算法鑒權
type HMACAuth struct {
	SecretKey []byte
}

// Sign 對給定Body生成expires後失效的簽名，expires為過期時間戳，
// 填寫為0表示不限制有效期
func (auth HMACAuth) Sign(body string, expires int64) string {
	h := hmac.New(sha256.New, auth.SecretKey)
	expireTimeStamp := strconv.FormatInt(expires, 10)
	_, err := io.WriteString(h, body+":"+expireTimeStamp)
	if err != nil {
		return ""
	}

	return base64.URLEncoding.EncodeToString(h.Sum(nil)) + ":" + expireTimeStamp
}

// Check 對給定Body和Sign進行鑒權，包括對expires的檢查
func (auth HMACAuth) Check(body string, sign string) error {
	signSlice := strings.Split(sign, ":")
	// 如果未攜帶expires欄位
	if signSlice[len(signSlice)-1] == "" {
		return ErrAuthFailed
	}

	// 驗證是否過期
	expires, err := strconv.ParseInt(signSlice[len(signSlice)-1], 10, 64)
	if err != nil {
		return ErrAuthFailed.WithError(err)
	}
	// 如果簽名過期
	if expires < time.Now().Unix() && expires != 0 {
		return ErrExpired
	}

	// 驗證簽名
	if auth.Sign(body, expires) != sign {
		return ErrAuthFailed
	}
	return nil
}
