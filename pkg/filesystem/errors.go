package filesystem

import (
	"errors"

	"github.com/cloudreve/Cloudreve/v3/pkg/serializer"
)

var (
	ErrUnknownPolicyType       = errors.New("未知儲存策略類型")
	ErrFileSizeTooBig          = errors.New("單個文件尺寸太大")
	ErrFileExtensionNotAllowed = errors.New("不允許上傳此類型的文件")
	ErrInsufficientCapacity    = errors.New("容量空間不足")
	ErrIllegalObjectName       = errors.New("目標名稱非法")
	ErrClientCanceled          = errors.New("用戶端取消操作")
	ErrRootProtected           = errors.New("無法對根目錄進行操作")
	ErrInsertFileRecord        = serializer.NewError(serializer.CodeDBError, "無法插入文件記錄", nil)
	ErrFileExisted             = serializer.NewError(serializer.CodeObjectExist, "同名文件或目錄已存在", nil)
	ErrFolderExisted           = serializer.NewError(serializer.CodeObjectExist, "同名目錄已存在", nil)
	ErrPathNotExist            = serializer.NewError(404, "路徑不存在", nil)
	ErrObjectNotExist          = serializer.NewError(404, "文件不存在", nil)
	ErrIO                      = serializer.NewError(serializer.CodeIOFailed, "無法讀取文件資料", nil)
	ErrDBListObjects           = serializer.NewError(serializer.CodeDBError, "無法列取物件記錄", nil)
	ErrDBDeleteObjects         = serializer.NewError(serializer.CodeDBError, "無法刪除物件記錄", nil)
)
