package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	model "github.com/cloudreve/Cloudreve/v3/models"
	"github.com/cloudreve/Cloudreve/v3/pkg/auth"
	"github.com/cloudreve/Cloudreve/v3/pkg/cache"
	"github.com/cloudreve/Cloudreve/v3/pkg/conf"
	"github.com/cloudreve/Cloudreve/v3/pkg/filesystem/driver/cos"
	"github.com/cloudreve/Cloudreve/v3/pkg/filesystem/driver/onedrive"
	"github.com/cloudreve/Cloudreve/v3/pkg/filesystem/driver/oss"
	"github.com/cloudreve/Cloudreve/v3/pkg/filesystem/driver/s3"
	"github.com/cloudreve/Cloudreve/v3/pkg/request"
	"github.com/cloudreve/Cloudreve/v3/pkg/serializer"
	"github.com/cloudreve/Cloudreve/v3/pkg/util"
	"github.com/gin-gonic/gin"
	cossdk "github.com/tencentyun/cos-go-sdk-v5"
)

// PathTestService 本機路徑測試服務
type PathTestService struct {
	Path string `json:"path" binding:"required"`
}

// SlaveTestService 從機測試服務
type SlaveTestService struct {
	Secret string `json:"secret" binding:"required"`
	Server string `json:"server" binding:"required"`
}

// SlavePingService 從機相應ping
type SlavePingService struct {
	Callback string `json:"callback" binding:"required"`
}

// AddPolicyService 儲存策略添加服務
type AddPolicyService struct {
	Policy model.Policy `json:"policy" binding:"required"`
}

// PolicyService 儲存策略ID服務
type PolicyService struct {
	ID     uint   `uri:"id" json:"id" binding:"required"`
	Region string `json:"region"`
}

// Delete 刪除儲存策略
func (service *PolicyService) Delete() serializer.Response {
	// 禁止刪除預設策略
	if service.ID == 1 {
		return serializer.Err(serializer.CodeNoPermissionErr, "預設儲存策略無法刪除", nil)
	}

	policy, err := model.GetPolicyByID(service.ID)
	if err != nil {
		return serializer.Err(serializer.CodeNotFound, "儲存策略不存在", err)
	}

	// 檢查是否有文件使用
	total := 0
	row := model.DB.Model(&model.File{}).Where("policy_id = ?", service.ID).
		Select("count(id)").Row()
	row.Scan(&total)
	if total > 0 {
		return serializer.ParamErr(fmt.Sprintf("有 %d 個文件仍在使用此儲存策略，請先刪除這些文件", total), nil)
	}

	// 檢查使用者群組使用
	var groups []model.Group
	model.DB.Model(&model.Group{}).Where(
		"policies like ?",
		fmt.Sprintf("%%[%d]%%", service.ID),
	).Find(&groups)

	if len(groups) > 0 {
		return serializer.ParamErr(fmt.Sprintf("有 %d 個使用者群組綁定了此儲存策略，請先解除綁定", len(groups)), nil)
	}

	model.DB.Delete(&policy)
	policy.ClearCache()

	return serializer.Response{}
}

// Get 獲取儲存策略詳情
func (service *PolicyService) Get() serializer.Response {
	policy, err := model.GetPolicyByID(service.ID)
	if err != nil {
		return serializer.Err(serializer.CodeNotFound, "儲存策略不存在", err)
	}

	return serializer.Response{Data: policy}
}

// GetOAuth 獲取 OneDrive OAuth 地址
func (service *PolicyService) GetOAuth(c *gin.Context) serializer.Response {
	policy, err := model.GetPolicyByID(service.ID)
	if err != nil || policy.Type != "onedrive" {
		return serializer.Err(serializer.CodeNotFound, "儲存策略不存在", nil)
	}

	client, err := onedrive.NewClient(&policy)
	if err != nil {
		return serializer.Err(serializer.CodeInternalSetting, "無法初始化 OneDrive 用戶端", err)
	}

	util.SetSession(c, map[string]interface{}{
		"onedrive_oauth_policy": policy.ID,
	})

	cache.Deletes([]string{policy.BucketName}, "onedrive_")

	return serializer.Response{Data: client.OAuthURL(context.Background(), []string{
		"offline_access",
		"files.readwrite.all",
	})}
}

// AddSCF 建立回調雲函數
func (service *PolicyService) AddSCF() serializer.Response {
	policy, err := model.GetPolicyByID(service.ID)
	if err != nil {
		return serializer.Err(serializer.CodeNotFound, "儲存策略不存在", nil)
	}

	if err := cos.CreateSCF(&policy, service.Region); err != nil {
		return serializer.Err(serializer.CodeInternalSetting, "雲函數建立失敗", err)
	}

	return serializer.Response{}
}

// AddCORS 建立跨域策略
func (service *PolicyService) AddCORS() serializer.Response {
	policy, err := model.GetPolicyByID(service.ID)
	if err != nil {
		return serializer.Err(serializer.CodeNotFound, "儲存策略不存在", nil)
	}

	switch policy.Type {
	case "oss":
		handler := oss.Driver{
			Policy:     &policy,
			HTTPClient: request.HTTPClient{},
		}
		if err := handler.CORS(); err != nil {
			return serializer.Err(serializer.CodeInternalSetting, "跨域策略添加失敗", err)
		}
	case "cos":
		u, _ := url.Parse(policy.Server)
		b := &cossdk.BaseURL{BucketURL: u}
		handler := cos.Driver{
			Policy:     &policy,
			HTTPClient: request.HTTPClient{},
			Client: cossdk.NewClient(b, &http.Client{
				Transport: &cossdk.AuthorizationTransport{
					SecretID:  policy.AccessKey,
					SecretKey: policy.SecretKey,
				},
			}),
		}
		if err := handler.CORS(); err != nil {
			return serializer.Err(serializer.CodeInternalSetting, "跨域策略添加失敗", err)
		}
	case "s3":
		handler := s3.Driver{
			Policy: &policy,
		}
		if err := handler.CORS(); err != nil {
			return serializer.Err(serializer.CodeInternalSetting, "跨域策略添加失敗", err)
		}
	default:
		return serializer.ParamErr("不支援此策略", nil)
	}

	return serializer.Response{}
}

// Test 從機響應ping
func (service *SlavePingService) Test() serializer.Response {
	master, err := url.Parse(service.Callback)
	if err != nil {
		return serializer.ParamErr("無法解析主機站點地址，請檢查主機 參數設定 - 站點訊息 - 站點URL設定，"+err.Error(), nil)
	}

	controller, _ := url.Parse("/api/v3/site/ping")

	r := request.HTTPClient{}
	res, err := r.Request(
		"GET",
		master.ResolveReference(controller).String(),
		nil,
		request.WithTimeout(time.Duration(10)*time.Second),
	).DecodeResponse()

	if err != nil {
		return serializer.ParamErr("從機無法向主機發送回調請求，請檢查主機端 參數設定 - 站點訊息 - 站點URL設定，並確保從機可以連接到此地址，"+err.Error(), nil)
	}

	if res.Data.(string) != conf.BackendVersion {
		return serializer.ParamErr("Cloudreve版本不一致，主機："+res.Data.(string)+"，從機："+conf.BackendVersion, nil)
	}

	return serializer.Response{}
}

// Test 測試從機通信
func (service *SlaveTestService) Test() serializer.Response {
	slave, err := url.Parse(service.Server)
	if err != nil {
		return serializer.ParamErr("無法解析從機端地址，"+err.Error(), nil)
	}

	controller, _ := url.Parse("/api/v3/slave/ping")

	// 請求正文
	body := map[string]string{
		"callback": model.GetSiteURL().String(),
	}
	bodyByte, _ := json.Marshal(body)

	r := request.HTTPClient{}
	res, err := r.Request(
		"POST",
		slave.ResolveReference(controller).String(),
		bytes.NewReader(bodyByte),
		request.WithTimeout(time.Duration(10)*time.Second),
		request.WithCredential(
			auth.HMACAuth{SecretKey: []byte(service.Secret)},
			int64(model.GetIntSetting("slave_api_timeout", 60)),
		),
	).DecodeResponse()
	if err != nil {
		return serializer.ParamErr("無連接到從機，"+err.Error(), nil)
	}

	if res.Code != 0 {
		return serializer.ParamErr("成功接到從機，但是"+res.Msg, nil)
	}

	return serializer.Response{}
}

// Add 添加儲存策略
func (service *AddPolicyService) Add() serializer.Response {
	if service.Policy.Type != "local" && service.Policy.Type != "remote" {
		service.Policy.DirNameRule = strings.TrimPrefix(service.Policy.DirNameRule, "/")
	}

	if service.Policy.ID > 0 {
		if err := model.DB.Save(&service.Policy).Error; err != nil {
			return serializer.ParamErr("儲存策略儲存失敗", err)
		}
	} else {
		if err := model.DB.Create(&service.Policy).Error; err != nil {
			return serializer.ParamErr("儲存策略添加失敗", err)
		}
	}

	service.Policy.ClearCache()

	return serializer.Response{Data: service.Policy.ID}
}

// Test 測試本機路徑
func (service *PathTestService) Test() serializer.Response {
	policy := model.Policy{DirNameRule: service.Path}
	path := policy.GeneratePath(1, "/My File")
	path = filepath.Join(path, "test.txt")
	file, err := util.CreatNestedFile(util.RelativePath(path))
	if err != nil {
		return serializer.ParamErr(fmt.Sprintf("無法建立路徑 %s , %s", path, err.Error()), nil)
	}

	file.Close()
	os.Remove(path)

	return serializer.Response{}
}

// Policies 列出儲存策略
func (service *AdminListService) Policies() serializer.Response {
	var res []model.Policy
	total := 0

	tx := model.DB.Model(&model.Policy{})
	if service.OrderBy != "" {
		tx = tx.Order(service.OrderBy)
	}

	for k, v := range service.Conditions {
		tx = tx.Where(k+" = ?", v)
	}

	// 計算總數用於分頁
	tx.Count(&total)

	// 查詢記錄
	tx.Limit(service.PageSize).Offset((service.Page - 1) * service.PageSize).Find(&res)

	// 統計每個策略的文件使用
	statics := make(map[uint][2]int, len(res))
	for i := 0; i < len(res); i++ {
		total := [2]int{}
		row := model.DB.Model(&model.File{}).Where("policy_id = ?", res[i].ID).
			Select("count(id),sum(size)").Row()
		row.Scan(&total[0], &total[1])
		statics[res[i].ID] = total
	}

	return serializer.Response{Data: map[string]interface{}{
		"total":   total,
		"items":   res,
		"statics": statics,
	}}
}
