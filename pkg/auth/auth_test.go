package auth

import (
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	"github.com/cloudreve/Cloudreve/v3/pkg/util"
	"github.com/stretchr/testify/assert"
)

func TestSignURI(t *testing.T) {
	asserts := assert.New(t)
	General = HMACAuth{SecretKey: []byte(util.RandStringRunes(256))}

	// 成功
	{
		sign, err := SignURI(General, "/api/v3/something?id=1", 0)
		asserts.NoError(err)
		queries := sign.Query()
		asserts.Equal("1", queries.Get("id"))
		asserts.NotEmpty(queries.Get("sign"))
	}

	// URI解碼失敗
	{
		sign, err := SignURI(General, "://dg.;'f]gh./'", 0)
		asserts.Error(err)
		asserts.Nil(sign)
	}
}

func TestCheckURI(t *testing.T) {
	asserts := assert.New(t)
	General = HMACAuth{SecretKey: []byte(util.RandStringRunes(256))}

	// 成功
	{
		sign, err := SignURI(General, "/api/ok?if=sdf&fd=go", 10)
		asserts.NoError(err)
		asserts.NoError(CheckURI(General, sign))
	}

	// 過期
	{
		sign, err := SignURI(General, "/api/ok?if=sdf&fd=go", -1)
		asserts.NoError(err)
		asserts.Error(CheckURI(General, sign))
	}
}

func TestSignRequest(t *testing.T) {
	asserts := assert.New(t)
	General = HMACAuth{SecretKey: []byte(util.RandStringRunes(256))}

	// 非上傳請求
	{
		req, err := http.NewRequest("POST", "http://127.0.0.1/api/v3/slave/upload", strings.NewReader("I am body."))
		asserts.NoError(err)
		req = SignRequest(General, req, 0)
		asserts.NotEmpty(req.Header["Authorization"])
	}

	// 上傳請求
	{
		req, err := http.NewRequest(
			"POST",
			"http://127.0.0.1/api/v3/slave/upload",
			strings.NewReader("I am body."),
		)
		asserts.NoError(err)
		req.Header["X-Policy"] = []string{"I am Policy"}
		req = SignRequest(General, req, 10)
		asserts.NotEmpty(req.Header["Authorization"])
	}
}

func TestCheckRequest(t *testing.T) {
	asserts := assert.New(t)
	General = HMACAuth{SecretKey: []byte(util.RandStringRunes(256))}

	// 非上傳請求 驗證成功
	{
		req, err := http.NewRequest(
			"POST",
			"http://127.0.0.1/api/v3/upload",
			strings.NewReader("I am body."),
		)
		asserts.NoError(err)
		req = SignRequest(General, req, 0)
		err = CheckRequest(General, req)
		asserts.NoError(err)
	}

	// 上傳請求 驗證成功
	{
		req, err := http.NewRequest(
			"POST",
			"http://127.0.0.1/api/v3/upload",
			strings.NewReader("I am body."),
		)
		asserts.NoError(err)
		req.Header["X-Policy"] = []string{"I am Policy"}
		req = SignRequest(General, req, 0)
		err = CheckRequest(General, req)
		asserts.NoError(err)
	}

	// 非上傳請求 失敗
	{
		req, err := http.NewRequest(
			"POST",
			"http://127.0.0.1/api/v3/upload",
			strings.NewReader("I am body."),
		)
		asserts.NoError(err)
		req = SignRequest(General, req, 0)
		req.Body = ioutil.NopCloser(strings.NewReader("2333"))
		err = CheckRequest(General, req)
		asserts.Error(err)
	}
}
