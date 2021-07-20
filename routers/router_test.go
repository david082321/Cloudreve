package routers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	model "github.com/cloudreve/Cloudreve/v3/models"
	"github.com/jinzhu/gorm"
	"github.com/stretchr/testify/assert"
)

func TestPing(t *testing.T) {
	asserts := assert.New(t)
	router := InitMasterRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v3/site/ping", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	asserts.Contains(w.Body.String(), "Pong")
}

func TestCaptcha(t *testing.T) {
	asserts := assert.New(t)
	router := InitMasterRouter()
	w := httptest.NewRecorder()

	req, _ := http.NewRequest(
		"GET",
		"/api/v3/site/captcha",
		nil,
	)

	router.ServeHTTP(w, req)

	asserts.Equal(200, w.Code)
	asserts.Contains(w.Body.String(), "base64")
}

//func TestUserSession(t *testing.T) {
//	mutex.Lock()
//	defer mutex.Unlock()
//	switchToMockDB()
//	asserts := assert.New(t)
//	router := InitMasterRouter()
//	w := httptest.NewRecorder()
//
//	// 建立測試用驗證碼
//	var configD = base64Captcha.ConfigDigit{
//		Height:     80,
//		Width:      240,
//		MaxSkew:    0.7,
//		DotCount:   80,
//		CaptchaLen: 1,
//	}
//	idKeyD, _ := base64Captcha.GenerateCaptcha("", configD)
//	middleware.ContextMock = map[string]interface{}{
//		"captchaID": idKeyD,
//	}
//
//	testCases := []struct {
//		settingRows *sqlmock.Rows
//		userRows    *sqlmock.Rows
//		policyRows  *sqlmock.Rows
//		reqBody     string
//		expected    interface{}
//	}{
//		// 登入訊息正確，不需要驗證碼
//		{
//			settingRows: sqlmock.NewRows([]string{"name", "value", "type"}).
//				AddRow("login_captcha", "0", "login"),
//			userRows: sqlmock.NewRows([]string{"email", "nick", "password", "options"}).
//				AddRow("admin@cloudreve.org", "admin", "CKLmDKa1C9SD64vU:76adadd4fd4bad86959155f6f7bc8993c94e7adf", "{}"),
//			expected: serializer.BuildUserResponse(model.User{
//				Email: "admin@cloudreve.org",
//				Nick:  "admin",
//				Policy: model.Policy{
//					Type:              "local",
//					OptionsSerialized: model.PolicyOption{FileType: []string{}},
//				},
//			}),
//		},
//		// 登入訊息正確，需要驗證碼,驗證碼錯誤
//		{
//			settingRows: sqlmock.NewRows([]string{"name", "value", "type"}).
//				AddRow("login_captcha", "1", "login"),
//			userRows: sqlmock.NewRows([]string{"email", "nick", "password", "options"}).
//				AddRow("admin@cloudreve.org", "admin", "CKLmDKa1C9SD64vU:76adadd4fd4bad86959155f6f7bc8993c94e7adf", "{}"),
//			expected: serializer.ParamErr("驗證碼錯誤", nil),
//		},
//		// 信箱正確密碼錯誤
//		{
//			settingRows: sqlmock.NewRows([]string{"name", "value", "type"}).
//				AddRow("login_captcha", "0", "login"),
//			userRows: sqlmock.NewRows([]string{"email", "nick", "password", "options"}).
//				AddRow("admin@cloudreve.org", "admin", "CKLmDKa1C9SD64vU:76adadd4fd4bad86959155f6f7bc8993c94e7adf", "{}"),
//			expected: serializer.Err(401, "使用者信箱或密碼錯誤", nil),
//		},
//		//信箱格式不正確
//		{
//			reqBody:  `{"userName":"admin@cloudreve","captchaCode":"captchaCode","Password":"admin123"}`,
//			expected: serializer.Err(40001, "信箱格式不正確", errors.New("Key: 'UserLoginService.UserName' Error:Field validation for 'UserName' failed on the 'email' tag")),
//		},
//		// 使用者被Ban
//		{
//			settingRows: sqlmock.NewRows([]string{"name", "value", "type"}).
//				AddRow("login_captcha", "0", "login"),
//			userRows: sqlmock.NewRows([]string{"email", "nick", "password", "options", "status"}).
//				AddRow("admin@cloudreve.org", "admin", "CKLmDKa1C9SD64vU:76adadd4fd4bad86959155f6f7bc8993c94e7adf", "{}", model.Baned),
//			expected: serializer.Err(403, "該帳號已被封禁", nil),
//		},
//		// 使用者未啟動
//		{
//			settingRows: sqlmock.NewRows([]string{"name", "value", "type"}).
//				AddRow("login_captcha", "0", "login"),
//			userRows: sqlmock.NewRows([]string{"email", "nick", "password", "options", "status"}).
//				AddRow("admin@cloudreve.org", "admin", "CKLmDKa1C9SD64vU:76adadd4fd4bad86959155f6f7bc8993c94e7adf", "{}", model.NotActivicated),
//			expected: serializer.Err(403, "該帳號未啟動", nil),
//		},
//	}
//
//	for k, testCase := range testCases {
//		if testCase.settingRows != nil {
//			mock.ExpectQuery("^SELECT (.+)").WillReturnRows(testCase.settingRows)
//		}
//		if testCase.userRows != nil {
//			mock.ExpectQuery("^SELECT (.+)").WillReturnRows(testCase.userRows)
//		}
//		if testCase.policyRows != nil {
//			mock.ExpectQuery("^SELECT \\* FROM `(.+)` WHERE `(.+)`\\.`deleted_at` IS NULL AND \\(\\(`policies`.`id` = 1\\)\\)(.+)$").WillReturnRows(testCase.policyRows)
//		}
//		req, _ := http.NewRequest(
//			"POST",
//			"/api/v3/user/session",
//			bytes.NewReader([]byte(testCase.reqBody)),
//		)
//		router.ServeHTTP(w, req)
//
//		asserts.Equal(200, w.Code)
//		expectedJSON, _ := json.Marshal(testCase.expected)
//		asserts.JSONEq(string(expectedJSON), w.Body.String(), "測試用例：%d", k)
//
//		w.Body.Reset()
//		asserts.NoError(mock.ExpectationsWereMet())
//		model.ClearCache()
//	}
//
//}
//
//func TestSessionAuthCheck(t *testing.T) {
//	mutex.Lock()
//	defer mutex.Unlock()
//	switchToMockDB()
//	asserts := assert.New(t)
//	router := InitMasterRouter()
//	w := httptest.NewRecorder()
//
//	mock.ExpectQuery("^SELECT (.+)").WillReturnRows(sqlmock.NewRows([]string{"email", "nick", "password", "options"}).
//		AddRow("admin@cloudreve.org", "admin", "CKLmDKa1C9SD64vU:76adadd4fd4bad86959155f6f7bc8993c94e7adf", "{}"))
//	expectedUser, _ := model.GetUserByID(1)
//
//	testCases := []struct {
//		userRows    *sqlmock.Rows
//		sessionMock map[string]interface{}
//		contextMock map[string]interface{}
//		expected    interface{}
//	}{
//		// 未登入
//		{
//			expected: serializer.CheckLogin(),
//		},
//		// 登入正常
//		{
//			userRows: sqlmock.NewRows([]string{"email", "nick", "password", "options"}).
//				AddRow("admin@cloudreve.org", "admin", "CKLmDKa1C9SD64vU:76adadd4fd4bad86959155f6f7bc8993c94e7adf", "{}"),
//			sessionMock: map[string]interface{}{"user_id": 1},
//			expected:    serializer.BuildUserResponse(expectedUser),
//		},
//		// UID不存在
//		{
//			userRows:    sqlmock.NewRows([]string{"email", "nick", "password", "options"}),
//			sessionMock: map[string]interface{}{"user_id": -1},
//			expected:    serializer.CheckLogin(),
//		},
//	}
//
//	for _, testCase := range testCases {
//		req, _ := http.NewRequest(
//			"GET",
//			"/api/v3/user/me",
//			nil,
//		)
//		if testCase.userRows != nil {
//			mock.ExpectQuery("^SELECT (.+)").WillReturnRows(testCase.userRows)
//		}
//		middleware.ContextMock = testCase.contextMock
//		middleware.SessionMock = testCase.sessionMock
//		router.ServeHTTP(w, req)
//		expectedJSON, _ := json.Marshal(testCase.expected)
//
//		asserts.Equal(200, w.Code)
//		asserts.JSONEq(string(expectedJSON), w.Body.String())
//		asserts.NoError(mock.ExpectationsWereMet())
//
//		w.Body.Reset()
//	}
//
//}

func TestSiteConfigRoute(t *testing.T) {
	switchToMemDB()
	asserts := assert.New(t)
	router := InitMasterRouter()
	w := httptest.NewRecorder()

	req, _ := http.NewRequest(
		"GET",
		"/api/v3/site/config",
		nil,
	)
	router.ServeHTTP(w, req)
	asserts.Equal(200, w.Code)
	asserts.Contains(w.Body.String(), "Cloudreve")

	w.Body.Reset()

	// 消除無效值
	model.DB.Model(&model.Setting{
		Model: gorm.Model{
			ID: 2,
		},
	}).UpdateColumn("name", "siteName_b")

	req, _ = http.NewRequest(
		"GET",
		"/api/v3/site/config",
		nil,
	)
	router.ServeHTTP(w, req)
	asserts.Equal(200, w.Code)
	asserts.Contains(w.Body.String(), "\"title\"")

	model.DB.Model(&model.Setting{
		Model: gorm.Model{
			ID: 2,
		},
	}).UpdateColumn("name", "siteName")
}
