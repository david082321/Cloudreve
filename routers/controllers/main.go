package controllers

import (
	"encoding/json"

	model "github.com/cloudreve/Cloudreve/v3/models"
	"github.com/cloudreve/Cloudreve/v3/pkg/serializer"
	"github.com/gin-gonic/gin"
	"gopkg.in/go-playground/validator.v9"
)

// ParamErrorMsg 根據Validator返回的錯誤訊息給出錯誤提示
func ParamErrorMsg(filed string, tag string) string {
	// 未通過驗證的表單域與中文對應
	fieldMap := map[string]string{
		"UserName": "信箱",
		"Password": "密碼",
		"Path":     "路徑",
		"SourceID": "原始資源",
		"URL":      "連結",
		"Nick":     "暱稱",
	}
	// 未透過的規則與中文對應
	tagMap := map[string]string{
		"required": "不能為空",
		"min":      "太短",
		"max":      "太長",
		"email":    "格式不正確",
	}
	fieldVal, findField := fieldMap[filed]
	tagVal, findTag := tagMap[tag]
	if findField && findTag {
		// 返回拼接出來的錯誤訊息
		return fieldVal + tagVal
	}
	return ""
}

// ErrorResponse 返回錯誤消息
func ErrorResponse(err error) serializer.Response {
	// 處理 Validator 產生的錯誤
	if ve, ok := err.(validator.ValidationErrors); ok {
		for _, e := range ve {
			return serializer.ParamErr(
				ParamErrorMsg(e.Field(), e.Tag()),
				err,
			)
		}
	}

	if _, ok := err.(*json.UnmarshalTypeError); ok {
		return serializer.ParamErr("JSON類型不匹配", err)
	}

	return serializer.ParamErr("參數錯誤", err)
}

// CurrentUser 獲取目前使用者
func CurrentUser(c *gin.Context) *model.User {
	if user, _ := c.Get("user"); user != nil {
		if u, ok := user.(*model.User); ok {
			return u
		}
	}
	return nil
}
