package request

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	"github.com/cloudreve/Cloudreve/v3/pkg/serializer"
	"github.com/stretchr/testify/assert"
	testMock "github.com/stretchr/testify/mock"
)

func TestRemoteCallback(t *testing.T) {
	asserts := assert.New(t)

	// 回調成功
	{
		clientMock := ClientMock{}
		mockResp, _ := json.Marshal(serializer.Response{Code: 0})
		clientMock.On(
			"Request",
			"POST",
			"http://test/test/url",
			testMock.Anything,
			testMock.Anything,
		).Return(&Response{
			Err: nil,
			Response: &http.Response{
				StatusCode: 200,
				Body:       ioutil.NopCloser(bytes.NewReader(mockResp)),
			},
		})
		GeneralClient = clientMock
		resp := RemoteCallback("http://test/test/url", serializer.UploadCallback{
			SourceName: "source",
		})
		asserts.NoError(resp)
		clientMock.AssertExpectations(t)
	}

	// 服務端返回業務錯誤
	{
		clientMock := ClientMock{}
		mockResp, _ := json.Marshal(serializer.Response{Code: 401})
		clientMock.On(
			"Request",
			"POST",
			"http://test/test/url",
			testMock.Anything,
			testMock.Anything,
		).Return(&Response{
			Err: nil,
			Response: &http.Response{
				StatusCode: 200,
				Body:       ioutil.NopCloser(bytes.NewReader(mockResp)),
			},
		})
		GeneralClient = clientMock
		resp := RemoteCallback("http://test/test/url", serializer.UploadCallback{
			SourceName: "source",
		})
		asserts.EqualValues(401, resp.(serializer.AppError).Code)
		clientMock.AssertExpectations(t)
	}

	// 無法解析回調響應
	{
		clientMock := ClientMock{}
		clientMock.On(
			"Request",
			"POST",
			"http://test/test/url",
			testMock.Anything,
			testMock.Anything,
		).Return(&Response{
			Err: nil,
			Response: &http.Response{
				StatusCode: 200,
				Body:       ioutil.NopCloser(strings.NewReader("mockResp")),
			},
		})
		GeneralClient = clientMock
		resp := RemoteCallback("http://test/test/url", serializer.UploadCallback{
			SourceName: "source",
		})
		asserts.Error(resp)
		clientMock.AssertExpectations(t)
	}

	// HTTP狀態碼非200
	{
		clientMock := ClientMock{}
		clientMock.On(
			"Request",
			"POST",
			"http://test/test/url",
			testMock.Anything,
			testMock.Anything,
		).Return(&Response{
			Err: nil,
			Response: &http.Response{
				StatusCode: 404,
				Body:       ioutil.NopCloser(strings.NewReader("mockResp")),
			},
		})
		GeneralClient = clientMock
		resp := RemoteCallback("http://test/test/url", serializer.UploadCallback{
			SourceName: "source",
		})
		asserts.Error(resp)
		clientMock.AssertExpectations(t)
	}

	// 無法發起回調
	{
		clientMock := ClientMock{}
		clientMock.On(
			"Request",
			"POST",
			"http://test/test/url",
			testMock.Anything,
			testMock.Anything,
		).Return(&Response{
			Err: errors.New("error"),
		})
		GeneralClient = clientMock
		resp := RemoteCallback("http://test/test/url", serializer.UploadCallback{
			SourceName: "source",
		})
		asserts.Error(resp)
		clientMock.AssertExpectations(t)
	}
}
