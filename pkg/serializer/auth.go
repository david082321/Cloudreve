package serializer

import "encoding/json"

// RequestRawSign 待簽名的HTTP請求
type RequestRawSign struct {
	Path   string
	Policy string
	Body   string
}

// NewRequestSignString 返回JSON格式的待簽名字串
// TODO 測試
func NewRequestSignString(path, policy, body string) string {
	req := RequestRawSign{
		Path:   path,
		Policy: policy,
		Body:   body,
	}
	res, _ := json.Marshal(req)
	return string(res)
}
