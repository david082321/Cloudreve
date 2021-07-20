package fsctx

type key int

const (
	// GinCtx Gin的上下文
	GinCtx key = iota
	// SavePathCtx 文件物理路徑
	SavePathCtx
	// FileHeaderCtx 上傳的文件
	FileHeaderCtx
	// PathCtx 文件或目錄的虛擬路徑
	PathCtx
	// FileModelCtx 文件資料庫模型
	FileModelCtx
	// FolderModelCtx 目錄資料庫模型
	FolderModelCtx
	// HTTPCtx HTTP請求的上下文
	HTTPCtx
	// UploadPolicyCtx 上傳策略，一般為slave模式下使用
	UploadPolicyCtx
	// UserCtx 使用者
	UserCtx
	// ThumbSizeCtx 縮圖尺寸
	ThumbSizeCtx
	// FileSizeCtx 檔案大小
	FileSizeCtx
	// ShareKeyCtx 分享文件的 HashID
	ShareKeyCtx
	// LimitParentCtx 限制父目錄
	LimitParentCtx
	// IgnoreDirectoryConflictCtx 忽略目錄重名衝突
	IgnoreDirectoryConflictCtx
	// RetryCtx 失敗重試次數
	RetryCtx
	// ForceUsePublicEndpointCtx 強制使用公網 Endpoint
	ForceUsePublicEndpointCtx
	// CancelFuncCtx Context 取消函數
	CancelFuncCtx
	// ValidateCapacityOnceCtx 限定歸還容量的操作只執行一次
	ValidateCapacityOnceCtx
	// 禁止上傳時同名覆蓋操作
	DisableOverwrite
)
