package onedrive

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	model "github.com/cloudreve/Cloudreve/v3/models"
	"github.com/cloudreve/Cloudreve/v3/pkg/cache"
	"github.com/cloudreve/Cloudreve/v3/pkg/request"
	"github.com/jinzhu/gorm"
	"github.com/stretchr/testify/assert"
	testMock "github.com/stretchr/testify/mock"
)

var mock sqlmock.Sqlmock

// TestMain 初始化資料庫Mock
func TestMain(m *testing.M) {
	var db *sql.DB
	var err error
	db, mock, err = sqlmock.New()
	if err != nil {
		panic("An error was not expected when opening a stub database connection")
	}
	model.DB, _ = gorm.Open("mysql", db)
	defer db.Close()
	m.Run()
}

func TestGetOAuthEndpoint(t *testing.T) {
	asserts := assert.New(t)

	// URL解析失敗
	{
		client := Client{
			Endpoints: &Endpoints{
				OAuthURL: string([]byte{0x7f}),
			},
		}
		res := client.getOAuthEndpoint()
		asserts.Nil(res)
	}

	{
		testCase := []struct {
			OAuthURL string
			token    string
			auth     string
			isChina  bool
		}{
			{
				OAuthURL: "http://login.live.com",
				token:    "https://login.live.com/oauth20_token.srf",
				auth:     "https://login.live.com/oauth20_authorize.srf",
				isChina:  false,
			},
			{
				OAuthURL: "http://login.chinacloudapi.cn",
				token:    "https://login.chinacloudapi.cn/common/oauth2/v2.0/token",
				auth:     "https://login.chinacloudapi.cn/common/oauth2/v2.0/authorize",
				isChina:  true,
			},
			{
				OAuthURL: "other",
				token:    "https://login.microsoftonline.com/common/oauth2/v2.0/token",
				auth:     "https://login.microsoftonline.com/common/oauth2/v2.0/authorize",
				isChina:  false,
			},
		}

		for i, testCase := range testCase {
			client := Client{
				Endpoints: &Endpoints{
					OAuthURL: testCase.OAuthURL,
				},
			}
			res := client.getOAuthEndpoint()
			asserts.Equal(testCase.token, res.token.String(), "Test Case #%d", i)
			asserts.Equal(testCase.auth, res.authorize.String(), "Test Case #%d", i)
			asserts.Equal(testCase.isChina, client.Endpoints.isInChina, "Test Case #%d", i)
		}
	}
}

func TestClient_OAuthURL(t *testing.T) {
	asserts := assert.New(t)

	client := Client{
		ClientID:  "client_id",
		Redirect:  "http://cloudreve.org/callback",
		Endpoints: &Endpoints{},
	}
	client.Endpoints.OAuthEndpoints = client.getOAuthEndpoint()
	res, err := url.Parse(client.OAuthURL(context.Background(), []string{"scope1", "scope2"}))
	asserts.NoError(err)
	query := res.Query()
	asserts.Equal("client_id", query.Get("client_id"))
	asserts.Equal("scope1 scope2", query.Get("scope"))
	asserts.Equal(client.Redirect, query.Get("redirect_uri"))

}

type ClientMock struct {
	testMock.Mock
}

func (m ClientMock) Request(method, target string, body io.Reader, opts ...request.Option) *request.Response {
	args := m.Called(method, target, body, opts)
	return args.Get(0).(*request.Response)
}

type mockReader string

func (r mockReader) Read(b []byte) (int, error) {
	return 0, errors.New("read error")
}

func TestClient_ObtainToken(t *testing.T) {
	asserts := assert.New(t)

	client := Client{
		Endpoints:    &Endpoints{},
		ClientID:     "ClientID",
		ClientSecret: "ClientSecret",
		Redirect:     "Redirect",
	}
	client.Endpoints.OAuthEndpoints = client.getOAuthEndpoint()

	// 重新整理Token 成功
	{
		clientMock := ClientMock{}
		clientMock.On(
			"Request",
			"POST",
			client.Endpoints.OAuthEndpoints.token.String(),
			testMock.Anything,
			testMock.Anything,
		).Return(&request.Response{
			Err: nil,
			Response: &http.Response{
				StatusCode: 200,
				Body:       ioutil.NopCloser(strings.NewReader(`{"access_token":"i am token"}`)),
			},
		})
		client.Request = clientMock

		res, err := client.ObtainToken(context.Background())
		clientMock.AssertExpectations(t)
		asserts.NoError(err)
		asserts.NotNil(res)
		asserts.Equal("i am token", res.AccessToken)
	}

	// 重新獲取 無法發送請求
	{
		clientMock := ClientMock{}
		clientMock.On(
			"Request",
			"POST",
			client.Endpoints.OAuthEndpoints.token.String(),
			testMock.Anything,
			testMock.Anything,
		).Return(&request.Response{
			Err: errors.New("error"),
		})
		client.Request = clientMock

		res, err := client.ObtainToken(context.Background(), WithCode("code"))
		clientMock.AssertExpectations(t)
		asserts.Error(err)
		asserts.Nil(res)
	}

	// 重新整理Token 無法獲取響應正文
	{
		clientMock := ClientMock{}
		clientMock.On(
			"Request",
			"POST",
			client.Endpoints.OAuthEndpoints.token.String(),
			testMock.Anything,
			testMock.Anything,
		).Return(&request.Response{
			Err: nil,
			Response: &http.Response{
				StatusCode: 200,
				Body:       ioutil.NopCloser(mockReader("")),
			},
		})
		client.Request = clientMock

		res, err := client.ObtainToken(context.Background())
		clientMock.AssertExpectations(t)
		asserts.Error(err)
		asserts.Nil(res)
		asserts.Equal("read error", err.Error())
	}

	// 重新整理Token OneDrive返回錯誤
	{
		clientMock := ClientMock{}
		clientMock.On(
			"Request",
			"POST",
			client.Endpoints.OAuthEndpoints.token.String(),
			testMock.Anything,
			testMock.Anything,
		).Return(&request.Response{
			Err: nil,
			Response: &http.Response{
				StatusCode: 400,
				Body:       ioutil.NopCloser(strings.NewReader(`{"error":"i am error"}`)),
			},
		})
		client.Request = clientMock

		res, err := client.ObtainToken(context.Background())
		clientMock.AssertExpectations(t)
		asserts.Error(err)
		asserts.Nil(res)
		asserts.Equal("", err.Error())
	}

	// 重新整理Token OneDrive未知響應
	{
		clientMock := ClientMock{}
		clientMock.On(
			"Request",
			"POST",
			client.Endpoints.OAuthEndpoints.token.String(),
			testMock.Anything,
			testMock.Anything,
		).Return(&request.Response{
			Err: nil,
			Response: &http.Response{
				StatusCode: 400,
				Body:       ioutil.NopCloser(strings.NewReader(`？？？`)),
			},
		})
		client.Request = clientMock

		res, err := client.ObtainToken(context.Background())
		clientMock.AssertExpectations(t)
		asserts.Error(err)
		asserts.Nil(res)
	}
}

func TestClient_UpdateCredential(t *testing.T) {
	asserts := assert.New(t)
	client := Client{
		Policy:       &model.Policy{Model: gorm.Model{ID: 257}},
		Endpoints:    &Endpoints{},
		ClientID:     "TestClient_UpdateCredential",
		ClientSecret: "ClientSecret",
		Redirect:     "Redirect",
		Credential:   &Credential{},
	}
	client.Endpoints.OAuthEndpoints = client.getOAuthEndpoint()

	// 無有效的RefreshToken
	{
		err := client.UpdateCredential(context.Background())
		asserts.Equal(ErrInvalidRefreshToken, err)
		client.Credential = nil
		err = client.UpdateCredential(context.Background())
		asserts.Equal(ErrInvalidRefreshToken, err)
	}

	// 成功
	{
		clientMock := ClientMock{}
		clientMock.On(
			"Request",
			"POST",
			client.Endpoints.OAuthEndpoints.token.String(),
			testMock.Anything,
			testMock.Anything,
		).Return(&request.Response{
			Err: nil,
			Response: &http.Response{
				StatusCode: 200,
				Body:       ioutil.NopCloser(strings.NewReader(`{"expires_in":3600,"refresh_token":"new_refresh_token","access_token":"i am token"}`)),
			},
		})
		client.Request = clientMock
		client.Credential = &Credential{
			RefreshToken: "old_refresh_token",
		}
		mock.ExpectBegin()
		mock.ExpectExec("UPDATE(.+)").WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()
		err := client.UpdateCredential(context.Background())
		clientMock.AssertExpectations(t)
		asserts.NoError(mock.ExpectationsWereMet())
		asserts.NoError(err)
		cacheRes, ok := cache.Get("onedrive_TestClient_UpdateCredential")
		asserts.True(ok)
		cacheCredential := cacheRes.(Credential)
		asserts.Equal("new_refresh_token", cacheCredential.RefreshToken)
		asserts.Equal("i am token", cacheCredential.AccessToken)
	}

	// OneDrive返回錯誤
	{
		cache.Deletes([]string{"TestClient_UpdateCredential"}, "onedrive_")
		clientMock := ClientMock{}
		clientMock.On(
			"Request",
			"POST",
			client.Endpoints.OAuthEndpoints.token.String(),
			testMock.Anything,
			testMock.Anything,
		).Return(&request.Response{
			Err: nil,
			Response: &http.Response{
				StatusCode: 400,
				Body:       ioutil.NopCloser(strings.NewReader(`{"error":"error"}`)),
			},
		})
		client.Request = clientMock
		client.Credential = &Credential{
			RefreshToken: "old_refresh_token",
		}
		err := client.UpdateCredential(context.Background())
		clientMock.AssertExpectations(t)
		asserts.Error(err)
	}

	// 從快取中獲取
	{
		cache.Set("onedrive_TestClient_UpdateCredential", Credential{
			ExpiresIn:    time.Now().Add(time.Duration(10) * time.Second).Unix(),
			AccessToken:  "AccessToken",
			RefreshToken: "RefreshToken",
		}, 0)
		client.Credential = &Credential{
			RefreshToken: "old_refresh_token",
		}
		err := client.UpdateCredential(context.Background())
		asserts.NoError(err)
		asserts.Equal("AccessToken", client.Credential.AccessToken)
		asserts.Equal("RefreshToken", client.Credential.RefreshToken)
	}

	// 無需重新獲取
	{
		client.Credential = &Credential{
			RefreshToken: "old_refresh_token",
			AccessToken:  "AccessToken2",
			ExpiresIn:    time.Now().Add(time.Duration(10) * time.Second).Unix(),
		}
		err := client.UpdateCredential(context.Background())
		asserts.NoError(err)
		asserts.Equal("AccessToken2", client.Credential.AccessToken)
	}
}
