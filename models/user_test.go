package model

import (
	"encoding/json"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/cloudreve/Cloudreve/v3/pkg/cache"
	"github.com/jinzhu/gorm"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func TestGetUserByID(t *testing.T) {
	asserts := assert.New(t)
	cache.Deletes([]string{"1"}, "policy_")
	//找到使用者時
	userRows := sqlmock.NewRows([]string{"id", "deleted_at", "email", "options", "group_id"}).
		AddRow(1, nil, "admin@cloudreve.org", "{}", 1)
	mock.ExpectQuery("^SELECT (.+)").WillReturnRows(userRows)

	groupRows := sqlmock.NewRows([]string{"id", "name", "policies"}).
		AddRow(1, "管理員", "[1]")
	mock.ExpectQuery("^SELECT (.+)").WillReturnRows(groupRows)

	policyRows := sqlmock.NewRows([]string{"id", "name"}).
		AddRow(1, "預設儲存策略")
	mock.ExpectQuery("^SELECT (.+)").WillReturnRows(policyRows)

	user, err := GetUserByID(1)
	asserts.NoError(err)
	asserts.Equal(User{
		Model: gorm.Model{
			ID:        1,
			DeletedAt: nil,
		},
		Email:   "admin@cloudreve.org",
		Options: "{}",
		GroupID: 1,
		Group: Group{
			Model: gorm.Model{
				ID: 1,
			},
			Name:       "管理員",
			Policies:   "[1]",
			PolicyList: []uint{1},
		},
		Policy: Policy{
			Model: gorm.Model{
				ID: 1,
			},
			OptionsSerialized: PolicyOption{
				FileType: []string{},
			},
			Name: "預設儲存策略",
		},
	}, user)

	//未找到使用者時
	mock.ExpectQuery("^SELECT (.+)").WillReturnError(errors.New("not found"))
	user, err = GetUserByID(1)
	asserts.Error(err)
	asserts.Equal(User{}, user)
}

func TestGetActiveUserByID(t *testing.T) {
	asserts := assert.New(t)
	cache.Deletes([]string{"1"}, "policy_")
	//找到使用者時
	userRows := sqlmock.NewRows([]string{"id", "deleted_at", "email", "options", "group_id"}).
		AddRow(1, nil, "admin@cloudreve.org", "{}", 1)
	mock.ExpectQuery("^SELECT (.+)").WillReturnRows(userRows)

	groupRows := sqlmock.NewRows([]string{"id", "name", "policies"}).
		AddRow(1, "管理員", "[1]")
	mock.ExpectQuery("^SELECT (.+)").WillReturnRows(groupRows)

	policyRows := sqlmock.NewRows([]string{"id", "name"}).
		AddRow(1, "預設儲存策略")
	mock.ExpectQuery("^SELECT (.+)").WillReturnRows(policyRows)

	user, err := GetActiveUserByID(1)
	asserts.NoError(err)
	asserts.Equal(User{
		Model: gorm.Model{
			ID:        1,
			DeletedAt: nil,
		},
		Email:   "admin@cloudreve.org",
		Options: "{}",
		GroupID: 1,
		Group: Group{
			Model: gorm.Model{
				ID: 1,
			},
			Name:       "管理員",
			Policies:   "[1]",
			PolicyList: []uint{1},
		},
		Policy: Policy{
			Model: gorm.Model{
				ID: 1,
			},
			OptionsSerialized: PolicyOption{
				FileType: []string{},
			},
			Name: "預設儲存策略",
		},
	}, user)

	//未找到使用者時
	mock.ExpectQuery("^SELECT (.+)").WillReturnError(errors.New("not found"))
	user, err = GetActiveUserByID(1)
	asserts.Error(err)
	asserts.Equal(User{}, user)
}

func TestUser_SetPassword(t *testing.T) {
	asserts := assert.New(t)
	user := User{}
	err := user.SetPassword("Cause Sega does what nintendon't")
	asserts.NoError(err)
	asserts.NotEmpty(user.Password)
}

func TestUser_CheckPassword(t *testing.T) {
	asserts := assert.New(t)
	user := User{}
	err := user.SetPassword("Cause Sega does what nintendon't")
	asserts.NoError(err)

	//密碼正確
	res, err := user.CheckPassword("Cause Sega does what nintendon't")
	asserts.NoError(err)
	asserts.True(res)

	//密碼錯誤
	res, err = user.CheckPassword("Cause Sega does what Nintendon't")
	asserts.NoError(err)
	asserts.False(res)

	//密碼欄位為空
	user = User{}
	res, err = user.CheckPassword("Cause Sega does what nintendon't")
	asserts.Error(err)
	asserts.False(res)

	// 未知密碼類型
	user = User{}
	user.Password = "1:2:3"
	res, err = user.CheckPassword("Cause Sega does what nintendon't")
	asserts.Error(err)
	asserts.False(res)

	// V2密碼，錯誤
	user = User{}
	user.Password = "md5:2:3"
	res, err = user.CheckPassword("Cause Sega does what nintendon't")
	asserts.NoError(err)
	asserts.False(res)

	// V2密碼，正確
	user = User{}
	user.Password = "md5:d8446059f8846a2c111a7f53515665fb:sdshare"
	res, err = user.CheckPassword("admin")
	asserts.NoError(err)
	asserts.True(res)

}

func TestNewUser(t *testing.T) {
	asserts := assert.New(t)
	newUser := NewUser()
	asserts.IsType(User{}, newUser)
	asserts.Empty(newUser.Avatar)
}

func TestUser_AfterFind(t *testing.T) {
	asserts := assert.New(t)
	cache.Deletes([]string{"1"}, "policy_")

	policyRows := sqlmock.NewRows([]string{"id", "name"}).
		AddRow(1, "預設儲存策略")
	mock.ExpectQuery("^SELECT (.+)").WillReturnRows(policyRows)

	newUser := NewUser()
	err := newUser.AfterFind()
	err = newUser.BeforeSave()
	expected := UserOption{}
	err = json.Unmarshal([]byte(newUser.Options), &expected)

	asserts.NoError(err)
	asserts.NoError(mock.ExpectationsWereMet())
	asserts.Equal(expected, newUser.OptionsSerialized)
	asserts.Equal("預設儲存策略", newUser.Policy.Name)
}

func TestUser_BeforeSave(t *testing.T) {
	asserts := assert.New(t)

	newUser := NewUser()
	err := newUser.BeforeSave()
	expected, err := json.Marshal(newUser.OptionsSerialized)

	asserts.NoError(err)
	asserts.Equal(string(expected), newUser.Options)
}

func TestUser_GetPolicyID(t *testing.T) {
	asserts := assert.New(t)

	newUser := NewUser()
	newUser.Group.PolicyList = []uint{1}

	asserts.EqualValues(1, newUser.GetPolicyID(0))

	newUser.Group.PolicyList = nil
	asserts.EqualValues(0, newUser.GetPolicyID(0))

	newUser.Group.PolicyList = []uint{}
	asserts.EqualValues(0, newUser.GetPolicyID(0))
}

func TestUser_GetRemainingCapacity(t *testing.T) {
	asserts := assert.New(t)
	newUser := NewUser()
	cache.Set("pack_size_0", uint64(0), 0)

	newUser.Group.MaxStorage = 100
	asserts.Equal(uint64(100), newUser.GetRemainingCapacity())

	newUser.Group.MaxStorage = 100
	newUser.Storage = 1
	asserts.Equal(uint64(99), newUser.GetRemainingCapacity())

	newUser.Group.MaxStorage = 100
	newUser.Storage = 100
	asserts.Equal(uint64(0), newUser.GetRemainingCapacity())

	newUser.Group.MaxStorage = 100
	newUser.Storage = 200
	asserts.Equal(uint64(0), newUser.GetRemainingCapacity())

	cache.Set("pack_size_0", uint64(10), 0)
	newUser.Group.MaxStorage = 100
	newUser.Storage = 101
	asserts.Equal(uint64(9), newUser.GetRemainingCapacity())
}

func TestUser_DeductionCapacity(t *testing.T) {
	asserts := assert.New(t)

	cache.Deletes([]string{"1"}, "policy_")
	userRows := sqlmock.NewRows([]string{"id", "deleted_at", "storage", "options", "group_id"}).
		AddRow(1, nil, 0, "{}", 1)
	mock.ExpectQuery("^SELECT (.+)").WillReturnRows(userRows)
	groupRows := sqlmock.NewRows([]string{"id", "name", "policies"}).
		AddRow(1, "管理員", "[1]")
	mock.ExpectQuery("^SELECT (.+)").WillReturnRows(groupRows)

	policyRows := sqlmock.NewRows([]string{"id", "name"}).
		AddRow(1, "預設儲存策略")
	mock.ExpectQuery("^SELECT (.+)").WillReturnRows(policyRows)

	newUser, err := GetUserByID(1)
	newUser.Group.MaxStorage = 100
	cache.Set("pack_size_1", uint64(0), 0)
	asserts.NoError(err)
	asserts.NoError(mock.ExpectationsWereMet())

	asserts.Equal(false, newUser.IncreaseStorage(101))
	asserts.Equal(uint64(0), newUser.Storage)

	asserts.Equal(true, newUser.IncreaseStorage(1))
	asserts.Equal(uint64(1), newUser.Storage)

	asserts.Equal(true, newUser.IncreaseStorage(99))
	asserts.Equal(uint64(100), newUser.Storage)

	asserts.Equal(false, newUser.IncreaseStorage(1))
	asserts.Equal(uint64(100), newUser.Storage)

	cache.Set("pack_size_1", uint64(1), 0)
	asserts.Equal(true, newUser.IncreaseStorage(1))
	asserts.Equal(uint64(101), newUser.Storage)

	asserts.True(newUser.IncreaseStorage(0))
}

func TestUser_DeductionStorage(t *testing.T) {
	asserts := assert.New(t)

	// 減少零
	{
		user := User{Storage: 1}
		asserts.True(user.DeductionStorage(0))
		asserts.Equal(uint64(1), user.Storage)
	}
	// 正常
	{
		user := User{
			Model:   gorm.Model{ID: 1},
			Storage: 10,
		}
		mock.ExpectBegin()
		mock.ExpectExec("UPDATE(.+)").WithArgs(5, sqlmock.AnyArg(), 1).WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()

		asserts.True(user.DeductionStorage(5))
		asserts.NoError(mock.ExpectationsWereMet())
		asserts.Equal(uint64(5), user.Storage)
	}

	// 減少的超出可用的
	{
		user := User{
			Model:   gorm.Model{ID: 1},
			Storage: 10,
		}
		mock.ExpectBegin()
		mock.ExpectExec("UPDATE(.+)").WithArgs(0, sqlmock.AnyArg(), 1).WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()

		asserts.False(user.DeductionStorage(20))
		asserts.NoError(mock.ExpectationsWereMet())
		asserts.Equal(uint64(0), user.Storage)
	}
}

func TestUser_IncreaseStorageWithoutCheck(t *testing.T) {
	asserts := assert.New(t)

	// 增加零
	{
		user := User{}
		user.IncreaseStorageWithoutCheck(0)
		asserts.Equal(uint64(0), user.Storage)
	}

	// 減少零
	{
		user := User{
			Model: gorm.Model{ID: 1},
		}
		mock.ExpectBegin()
		mock.ExpectExec("UPDATE(.+)").WithArgs(10, sqlmock.AnyArg(), 1).WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()

		user.IncreaseStorageWithoutCheck(10)
		asserts.NoError(mock.ExpectationsWereMet())
		asserts.Equal(uint64(10), user.Storage)
	}
}

func TestGetActiveUserByEmail(t *testing.T) {
	asserts := assert.New(t)

	mock.ExpectQuery("SELECT(.+)").WithArgs(Active, "abslant@foxmail.com").WillReturnRows(sqlmock.NewRows([]string{"id", "email"}))
	_, err := GetActiveUserByEmail("abslant@foxmail.com")

	asserts.Error(err)
	asserts.NoError(mock.ExpectationsWereMet())
}

func TestGetUserByEmail(t *testing.T) {
	asserts := assert.New(t)

	mock.ExpectQuery("SELECT(.+)").WithArgs("abslant@foxmail.com").WillReturnRows(sqlmock.NewRows([]string{"id", "email"}))
	_, err := GetUserByEmail("abslant@foxmail.com")

	asserts.Error(err)
	asserts.NoError(mock.ExpectationsWereMet())
}

func TestUser_AfterCreate(t *testing.T) {
	asserts := assert.New(t)
	user := User{Model: gorm.Model{ID: 1}}
	mock.ExpectBegin()
	mock.ExpectExec("INSERT(.+)").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	err := user.AfterCreate(DB)
	asserts.NoError(err)
	asserts.NoError(mock.ExpectationsWereMet())
}

func TestUser_Root(t *testing.T) {
	asserts := assert.New(t)
	user := User{Model: gorm.Model{ID: 1}}

	// 根目錄存在
	{
		mock.ExpectQuery("SELECT(.+)").WithArgs(1).WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(1, "根目錄"))
		root, err := user.Root()
		asserts.NoError(mock.ExpectationsWereMet())
		asserts.NoError(err)
		asserts.Equal("根目錄", root.Name)
	}

	// 根目錄不存在
	{
		mock.ExpectQuery("SELECT(.+)").WithArgs(1).WillReturnRows(sqlmock.NewRows([]string{"id", "name"}))
		_, err := user.Root()
		asserts.NoError(mock.ExpectationsWereMet())
		asserts.Error(err)
	}
}

func TestNewAnonymousUser(t *testing.T) {
	asserts := assert.New(t)

	mock.ExpectQuery("SELECT(.+)").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(3))
	user := NewAnonymousUser()
	asserts.NoError(mock.ExpectationsWereMet())
	asserts.NotNil(user)
	asserts.EqualValues(3, user.Group.ID)
}

func TestUser_IsAnonymous(t *testing.T) {
	asserts := assert.New(t)
	user := User{}
	asserts.True(user.IsAnonymous())
	user.ID = 1
	asserts.False(user.IsAnonymous())
}

func TestUser_SetStatus(t *testing.T) {
	asserts := assert.New(t)
	user := User{}
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE(.+)").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	user.SetStatus(Baned)
	asserts.NoError(mock.ExpectationsWereMet())
	asserts.Equal(Baned, user.Status)
}

func TestUser_UpdateOptions(t *testing.T) {
	asserts := assert.New(t)
	user := User{}

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE(.+)").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	asserts.NoError(user.UpdateOptions())
	asserts.NoError(mock.ExpectationsWereMet())
}
