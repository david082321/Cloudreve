package serializer

// RemoteDeleteRequest 遠端策略刪除介面請求正文
type RemoteDeleteRequest struct {
	Files []string `json:"files"`
}

// ListRequest 遠端策略列文件請求正文
type ListRequest struct {
	Path      string `json:"path"`
	Recursive bool   `json:"recursive"`
}
