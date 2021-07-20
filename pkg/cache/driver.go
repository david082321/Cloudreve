package cache

import (
	"github.com/cloudreve/Cloudreve/v3/pkg/conf"
	"github.com/gin-gonic/gin"
)

// Store 快取儲存器
var Store Driver = NewMemoStore()

// Init 初始化快取
func Init() {
	//Store = NewRedisStore(10, "tcp", "127.0.0.1:6379", "", "0")
	//return
	if conf.RedisConfig.Server != "" && gin.Mode() != gin.TestMode {
		Store = NewRedisStore(
			10,
			conf.RedisConfig.Network,
			conf.RedisConfig.Server,
			conf.RedisConfig.Password,
			conf.RedisConfig.DB,
		)
	}
}

// Driver 鍵值快取儲存容器
type Driver interface {
	// 設定值，ttl為過期時間，單位為秒
	Set(key string, value interface{}, ttl int) error

	// 取值，並返回是否成功
	Get(key string) (interface{}, bool)

	// 批次取值，返回成功取值的map即不存在的值
	Gets(keys []string, prefix string) (map[string]interface{}, []string)

	// 批次設定值，所有的key都會加上prefix前綴
	Sets(values map[string]interface{}, prefix string) error

	// 刪除值
	Delete(keys []string, prefix string) error
}

// Set 設定快取值
func Set(key string, value interface{}, ttl int) error {
	return Store.Set(key, value, ttl)
}

// Get 獲取快取值
func Get(key string) (interface{}, bool) {
	return Store.Get(key)
}

// Deletes 刪除值
func Deletes(keys []string, prefix string) error {
	return Store.Delete(keys, prefix)
}

// GetSettings 根據名稱批次獲取設定項快取
func GetSettings(keys []string, prefix string) (map[string]string, []string) {
	raw, miss := Store.Gets(keys, prefix)

	res := make(map[string]string, len(raw))
	for k, v := range raw {
		res[k] = v.(string)
	}

	return res, miss
}

// SetSettings 批次設定站點設定快取
func SetSettings(values map[string]string, prefix string) error {
	var toBeSet = make(map[string]interface{}, len(values))
	for key, value := range values {
		toBeSet[key] = interface{}(value)
	}
	return Store.Sets(toBeSet, prefix)
}
