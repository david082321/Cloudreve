package cache

import (
	"errors"
	"fmt"
	"github.com/gomodule/redigo/redis"
	"github.com/rafaeljusto/redigomock"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestNewRedisStore(t *testing.T) {
	asserts := assert.New(t)

	store := NewRedisStore(10, "tcp", "", "", "0")
	asserts.NotNil(store)

	conn, err := store.pool.Dial()
	asserts.Nil(conn)
	asserts.Error(err)

	testConn := redigomock.NewConn()
	cmd := testConn.Command("PING").Expect("PONG")
	err = store.pool.TestOnBorrow(testConn, time.Now())
	if testConn.Stats(cmd) != 1 {
		fmt.Println("Command was not used")
		return
	}
	asserts.NoError(err)
}

func TestRedisStore_Set(t *testing.T) {
	asserts := assert.New(t)
	conn := redigomock.NewConn()
	pool := &redis.Pool{
		Dial:    func() (redis.Conn, error) { return conn, nil },
		MaxIdle: 10,
	}
	store := &RedisStore{pool: pool}

	// 正常情況
	{
		cmd := conn.Command("SET", "test", redigomock.NewAnyData()).ExpectStringSlice("OK")
		err := store.Set("test", "test val", -1)
		asserts.NoError(err)
		if conn.Stats(cmd) != 1 {
			fmt.Println("Command was not used")
			return
		}
	}

	// 帶有TTL
	// 正常情況
	{
		cmd := conn.Command("SETEX", "test", 10, redigomock.NewAnyData()).ExpectStringSlice("OK")
		err := store.Set("test", "test val", 10)
		asserts.NoError(err)
		if conn.Stats(cmd) != 1 {
			fmt.Println("Command was not used")
			return
		}
	}

	// 序列化出錯
	{
		value := struct {
			Key string
		}{
			Key: "123",
		}
		err := store.Set("test", value, -1)
		asserts.Error(err)
	}

	// 指令執行失敗
	{
		conn.Clear()
		cmd := conn.Command("SET", "test", redigomock.NewAnyData()).ExpectError(errors.New("error"))
		err := store.Set("test", "test val", -1)
		asserts.Error(err)
		if conn.Stats(cmd) != 1 {
			fmt.Println("Command was not used")
			return
		}
	}
	// 獲取連線失敗
	{
		store.pool = &redis.Pool{
			Dial:    func() (redis.Conn, error) { return nil, errors.New("error") },
			MaxIdle: 10,
		}
		err := store.Set("test", "123", -1)
		asserts.Error(err)
	}

}

func TestRedisStore_Get(t *testing.T) {
	asserts := assert.New(t)
	conn := redigomock.NewConn()
	pool := &redis.Pool{
		Dial:    func() (redis.Conn, error) { return conn, nil },
		MaxIdle: 10,
	}
	store := &RedisStore{pool: pool}

	// 正常情況
	{
		expectVal, _ := serializer("test val")
		cmd := conn.Command("GET", "test").Expect(expectVal)
		val, ok := store.Get("test")
		if conn.Stats(cmd) != 1 {
			fmt.Println("Command was not used")
			return
		}
		asserts.True(ok)
		asserts.Equal("test val", val.(string))
	}

	// Key不存在
	{
		conn.Clear()
		cmd := conn.Command("GET", "test").Expect(nil)
		val, ok := store.Get("test")
		if conn.Stats(cmd) != 1 {
			fmt.Println("Command was not used")
			return
		}
		asserts.False(ok)
		asserts.Nil(val)
	}
	// 解碼錯誤
	{
		conn.Clear()
		cmd := conn.Command("GET", "test").Expect([]byte{0x20})
		val, ok := store.Get("test")
		if conn.Stats(cmd) != 1 {
			fmt.Println("Command was not used")
			return
		}
		asserts.False(ok)
		asserts.Nil(val)
	}
	// 獲取連線失敗
	{
		store.pool = &redis.Pool{
			Dial:    func() (redis.Conn, error) { return nil, errors.New("error") },
			MaxIdle: 10,
		}
		val, ok := store.Get("test")
		asserts.False(ok)
		asserts.Nil(val)
	}
}

func TestRedisStore_Gets(t *testing.T) {
	asserts := assert.New(t)
	conn := redigomock.NewConn()
	pool := &redis.Pool{
		Dial:    func() (redis.Conn, error) { return conn, nil },
		MaxIdle: 10,
	}
	store := &RedisStore{pool: pool}

	// 全部命中
	{
		conn.Clear()
		value1, _ := serializer("1")
		value2, _ := serializer("2")
		cmd := conn.Command("MGET", "test_1", "test_2").ExpectSlice(
			value1, value2)
		res, missed := store.Gets([]string{"1", "2"}, "test_")
		if conn.Stats(cmd) != 1 {
			fmt.Println("Command was not used")
			return
		}
		asserts.Len(missed, 0)
		asserts.Len(res, 2)
		asserts.Equal("1", res["1"].(string))
		asserts.Equal("2", res["2"].(string))
	}

	// 命中一個
	{
		conn.Clear()
		value2, _ := serializer("2")
		cmd := conn.Command("MGET", "test_1", "test_2").ExpectSlice(
			nil, value2)
		res, missed := store.Gets([]string{"1", "2"}, "test_")
		if conn.Stats(cmd) != 1 {
			fmt.Println("Command was not used")
			return
		}
		asserts.Len(missed, 1)
		asserts.Len(res, 1)
		asserts.Equal("1", missed[0])
		asserts.Equal("2", res["2"].(string))
	}

	// 指令出錯
	{
		conn.Clear()
		cmd := conn.Command("MGET", "test_1", "test_2").ExpectError(errors.New("error"))
		res, missed := store.Gets([]string{"1", "2"}, "test_")
		if conn.Stats(cmd) != 1 {
			fmt.Println("Command was not used")
			return
		}
		asserts.Len(missed, 2)
		asserts.Len(res, 0)
	}

	// 連接出錯
	{
		conn.Clear()
		store.pool = &redis.Pool{
			Dial:    func() (redis.Conn, error) { return nil, errors.New("error") },
			MaxIdle: 10,
		}
		res, missed := store.Gets([]string{"1", "2"}, "test_")
		asserts.Len(missed, 2)
		asserts.Len(res, 0)
	}
}

func TestRedisStore_Sets(t *testing.T) {
	asserts := assert.New(t)
	conn := redigomock.NewConn()
	pool := &redis.Pool{
		Dial:    func() (redis.Conn, error) { return conn, nil },
		MaxIdle: 10,
	}
	store := &RedisStore{pool: pool}

	// 正常
	{
		cmd := conn.Command("MSET", redigomock.NewAnyData(), redigomock.NewAnyData(), redigomock.NewAnyData(), redigomock.NewAnyData()).ExpectSlice("OK")
		err := store.Sets(map[string]interface{}{"1": "1", "2": "2"}, "test_")
		asserts.NoError(err)
		if conn.Stats(cmd) != 1 {
			fmt.Println("Command was not used")
			return
		}
	}

	// 序列化失敗
	{
		conn.Clear()
		value := struct {
			Key string
		}{
			Key: "123",
		}
		err := store.Sets(map[string]interface{}{"1": value, "2": "2"}, "test_")
		asserts.Error(err)
	}

	// 執行失敗
	{
		cmd := conn.Command("MSET", redigomock.NewAnyData(), redigomock.NewAnyData(), redigomock.NewAnyData(), redigomock.NewAnyData()).ExpectError(errors.New("error"))
		err := store.Sets(map[string]interface{}{"1": "1", "2": "2"}, "test_")
		asserts.Error(err)
		if conn.Stats(cmd) != 1 {
			fmt.Println("Command was not used")
			return
		}
	}

	// 連線失敗
	{
		conn.Clear()
		store.pool = &redis.Pool{
			Dial:    func() (redis.Conn, error) { return nil, errors.New("error") },
			MaxIdle: 10,
		}
		err := store.Sets(map[string]interface{}{"1": "1", "2": "2"}, "test_")
		asserts.Error(err)
	}
}

func TestRedisStore_Delete(t *testing.T) {
	asserts := assert.New(t)
	conn := redigomock.NewConn()
	pool := &redis.Pool{
		Dial:    func() (redis.Conn, error) { return conn, nil },
		MaxIdle: 10,
	}
	store := &RedisStore{pool: pool}

	// 正常
	{
		cmd := conn.Command("DEL", redigomock.NewAnyData(), redigomock.NewAnyData(), redigomock.NewAnyData(), redigomock.NewAnyData()).ExpectSlice("OK")
		err := store.Delete([]string{"1", "2", "3", "4"}, "test_")
		asserts.NoError(err)
		if conn.Stats(cmd) != 1 {
			fmt.Println("Command was not used")
			return
		}
	}

	// 指令執行失敗
	{
		conn.Clear()
		cmd := conn.Command("DEL", redigomock.NewAnyData(), redigomock.NewAnyData(), redigomock.NewAnyData(), redigomock.NewAnyData()).ExpectError(errors.New("error"))
		err := store.Delete([]string{"1", "2", "3", "4"}, "test_")
		asserts.Error(err)
		if conn.Stats(cmd) != 1 {
			fmt.Println("Command was not used")
			return
		}
	}

	// 連線失敗
	{
		conn.Clear()
		store.pool = &redis.Pool{
			Dial:    func() (redis.Conn, error) { return nil, errors.New("error") },
			MaxIdle: 10,
		}
		err := store.Delete([]string{"1", "2", "3", "4"}, "test_")
		asserts.Error(err)
	}
}
