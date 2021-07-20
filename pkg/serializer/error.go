package serializer

import "github.com/gin-gonic/gin"

// Response 基礎序列化器
type Response struct {
	Code  int         `json:"code"`
	Data  interface{} `json:"data,omitempty"`
	Msg   string      `json:"msg"`
	Error string      `json:"error,omitempty"`
}

// AppError 應用錯誤，實現了error介面
type AppError struct {
	Code     int
	Msg      string
	RawError error
}

// NewError 返回新的錯誤物件 todo:測試 還有下面的
func NewError(code int, msg string, err error) AppError {
	return AppError{
		Code:     code,
		Msg:      msg,
		RawError: err,
	}
}

// WithError 將應用error攜帶標準庫中的error
func (err *AppError) WithError(raw error) AppError {
	err.RawError = raw
	return *err
}

// Error 返回業務程式碼確定的可讀錯誤訊息
func (err AppError) Error() string {
	return err.Msg
}

// 三位數錯誤編碼為復用http原本含義
// 五位數錯誤編碼為應用自訂錯誤
// 五開頭的五位數錯誤編碼為伺服器端錯誤，比如資料庫操作失敗
// 四開頭的五位數錯誤編碼為用戶端錯誤，有時候是用戶端程式碼寫錯了，有時候是使用者操作錯誤
const (
	// CodeNotFullySuccess 未完全成功
	CodeNotFullySuccess = 203
	// CodeCheckLogin 未登入
	CodeCheckLogin = 401
	// CodeNoPermissionErr 未授權訪問
	CodeNoPermissionErr = 403
	// CodeNotFound 資源未找到
	CodeNotFound = 404
	// CodeUploadFailed 上傳出錯
	CodeUploadFailed = 40002
	// CodeCredentialInvalid 憑證無效
	CodeCredentialInvalid = 40001
	// CodeCreateFolderFailed 目錄建立失敗
	CodeCreateFolderFailed = 40003
	// CodeObjectExist 物件已存在
	CodeObjectExist = 40004
	// CodeSignExpired 簽名過期
	CodeSignExpired = 40005
	// CodePolicyNotAllowed 目前儲存策略不允許
	CodePolicyNotAllowed = 40006
	// CodeGroupNotAllowed 使用者群組無法進行此操作
	CodeGroupNotAllowed = 40007
	// CodeAdminRequired 非管理使用者群組
	CodeAdminRequired = 40008
	// CodeDBError 資料庫操作失敗
	CodeDBError = 50001
	// CodeEncryptError 加密失敗
	CodeEncryptError = 50002
	// CodeIOFailed IO操作失敗
	CodeIOFailed = 50004
	// CodeInternalSetting 內部設定參數錯誤
	CodeInternalSetting = 50005
	// CodeCacheOperation 快取操作失敗
	CodeCacheOperation = 50006
	// CodeCallbackError 回調失敗
	CodeCallbackError = 50007
	//CodeParamErr 各種奇奇怪怪的參數錯誤
	CodeParamErr = 40001
	// CodeNotSet 未定錯誤，後續嘗試從error中獲取
	CodeNotSet = -1
)

// DBErr 資料庫操作失敗
func DBErr(msg string, err error) Response {
	if msg == "" {
		msg = "資料庫操作失敗"
	}
	return Err(CodeDBError, msg, err)
}

// ParamErr 各種參數錯誤
func ParamErr(msg string, err error) Response {
	if msg == "" {
		msg = "參數錯誤"
	}
	return Err(CodeParamErr, msg, err)
}

// Err 通用錯誤處理
func Err(errCode int, msg string, err error) Response {
	// 底層錯誤是AppError，則嘗試從AppError中獲取詳細訊息
	if appError, ok := err.(AppError); ok {
		errCode = appError.Code
		err = appError.RawError
		msg = appError.Msg
	}

	res := Response{
		Code: errCode,
		Msg:  msg,
	}
	// 生產環境隱藏底層報錯
	if err != nil && gin.Mode() != gin.ReleaseMode {
		res.Error = err.Error()
	}
	return res
}
