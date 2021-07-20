package filesystem

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	model "github.com/cloudreve/Cloudreve/v3/models"
	"github.com/cloudreve/Cloudreve/v3/pkg/cache"
	"github.com/cloudreve/Cloudreve/v3/pkg/conf"
	"github.com/cloudreve/Cloudreve/v3/pkg/filesystem/fsctx"
	"github.com/cloudreve/Cloudreve/v3/pkg/filesystem/response"
	"github.com/cloudreve/Cloudreve/v3/pkg/serializer"
	"github.com/cloudreve/Cloudreve/v3/pkg/util"
	"github.com/jinzhu/gorm"
	"github.com/stretchr/testify/assert"
	testMock "github.com/stretchr/testify/mock"
)

func TestFileSystem_ListPhysical(t *testing.T) {
	asserts := assert.New(t)
	fs := &FileSystem{
		User: &model.User{
			Model: gorm.Model{
				ID: 1,
			},
		},
		Policy: &model.Policy{Type: "mock"},
	}
	ctx := context.Background()

	// 未知儲存策略
	{
		fs.Policy.Type = "unknown"
		res, err := fs.ListPhysical(ctx, "/")
		asserts.Equal(ErrUnknownPolicyType, err)
		asserts.Empty(res)
		fs.Policy.Type = "mock"
	}

	// 無法列取目錄
	{
		testHandler := new(FileHeaderMock)
		testHandler.On("List", testMock.Anything, "/", testMock.Anything).Return([]response.Object{}, errors.New("error"))
		fs.Handler = testHandler
		res, err := fs.ListPhysical(ctx, "/")
		asserts.EqualError(err, "error")
		asserts.Empty(res)
	}

	// 成功
	{
		testHandler := new(FileHeaderMock)
		testHandler.On("List", testMock.Anything, "/", testMock.Anything).Return(
			[]response.Object{{IsDir: true, Name: "1"}, {IsDir: false, Name: "2"}},
			nil,
		)
		fs.Handler = testHandler
		res, err := fs.ListPhysical(ctx, "/")
		asserts.NoError(err)
		asserts.Len(res, 1)
		asserts.Equal("1", res[0].Name)
	}
}

func TestFileSystem_List(t *testing.T) {
	asserts := assert.New(t)
	fs := &FileSystem{User: &model.User{
		Model: gorm.Model{
			ID: 1,
		},
	}}
	ctx := context.Background()

	// 成功，子目錄包含文件和路徑，不使用路徑處理鉤子
	// 根目錄
	mock.ExpectQuery("SELECT(.+)").
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "owner_id"}).AddRow(1, "/", 1))
	// folder
	mock.ExpectQuery("SELECT(.+)").
		WithArgs(1, 1, "folder").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "owner_id"}).AddRow(5, "folder", 1))

	mock.ExpectQuery("SELECT(.+)folder(.+)").WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(6, "sub_folder1").AddRow(7, "sub_folder2"))
	mock.ExpectQuery("SELECT(.+)file(.+)").WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(6, "sub_file1.txt").AddRow(7, "sub_file2.txt"))
	objects, err := fs.List(ctx, "/folder", nil)
	asserts.Len(objects, 4)
	asserts.NoError(err)
	asserts.NoError(mock.ExpectationsWereMet())

	// 成功，子目錄包含文件和路徑，不使用路徑處理鉤子，包含分享key
	// 根目錄
	mock.ExpectQuery("SELECT(.+)").
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "owner_id"}).AddRow(1, "/", 1))
	// folder
	mock.ExpectQuery("SELECT(.+)").
		WithArgs(1, 1, "folder").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "owner_id"}).AddRow(5, "folder", 1))

	mock.ExpectQuery("SELECT(.+)folder(.+)").WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(6, "sub_folder1").AddRow(7, "sub_folder2"))
	mock.ExpectQuery("SELECT(.+)file(.+)").WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(6, "sub_file1.txt").AddRow(7, "sub_file2.txt"))
	ctxWithKey := context.WithValue(ctx, fsctx.ShareKeyCtx, "share")
	objects, err = fs.List(ctxWithKey, "/folder", nil)
	asserts.Len(objects, 4)
	asserts.Equal("share", objects[3].Key)
	asserts.NoError(err)
	asserts.NoError(mock.ExpectationsWereMet())

	// 成功，子目錄包含文件和路徑，使用路徑處理鉤子
	mock.ExpectQuery("SELECT(.+)").
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "owner_id"}).AddRow(1, "/", 1))
	// folder
	mock.ExpectQuery("SELECT(.+)").
		WithArgs(1, 1, "folder").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "owner_id"}).AddRow(2, "folder", 1))

	mock.ExpectQuery("SELECT(.+)folder(.+)").WillReturnRows(sqlmock.NewRows([]string{"id", "name", "position"}).AddRow(6, "sub_folder1", "/folder").AddRow(7, "sub_folder2", "/folder"))
	mock.ExpectQuery("SELECT(.+)file(.+)").WillReturnRows(sqlmock.NewRows([]string{"id", "name", "dir"}).AddRow(6, "sub_file1.txt", "/folder").AddRow(7, "sub_file2.txt", "/folder"))
	objects, err = fs.List(ctx, "/folder", func(s string) string {
		return "prefix" + s
	})
	asserts.Len(objects, 4)
	asserts.NoError(err)
	asserts.NoError(mock.ExpectationsWereMet())
	for _, value := range objects {
		asserts.Contains(value.Path, "prefix/")
	}

	// 成功，子目錄包含路徑，使用路徑處理鉤子
	mock.ExpectQuery("SELECT(.+)").
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "owner_id"}).AddRow(1, "/", 1))
	// folder
	mock.ExpectQuery("SELECT(.+)").
		WithArgs(1, 1, "folder").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "owner_id"}).AddRow(2, "folder", 1))

	mock.ExpectQuery("SELECT(.+)folder(.+)").WillReturnRows(sqlmock.NewRows([]string{"id", "name", "position"}))
	mock.ExpectQuery("SELECT(.+)file(.+)").WillReturnRows(sqlmock.NewRows([]string{"id", "name", "dir"}).AddRow(6, "sub_file1.txt", "/folder").AddRow(7, "sub_file2.txt", "/folder"))
	objects, err = fs.List(ctx, "/folder", func(s string) string {
		return "prefix" + s
	})
	asserts.Len(objects, 2)
	asserts.NoError(err)
	asserts.NoError(mock.ExpectationsWereMet())
	for _, value := range objects {
		asserts.Contains(value.Path, "prefix/")
	}

	// 成功，子目錄下為空，使用路徑處理鉤子
	mock.ExpectQuery("SELECT(.+)").
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "owner_id"}).AddRow(1, "/", 1))
	// folder
	mock.ExpectQuery("SELECT(.+)").
		WithArgs(1, 1, "folder").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "owner_id"}).AddRow(2, "folder", 1))

	mock.ExpectQuery("SELECT(.+)folder(.+)").WillReturnRows(sqlmock.NewRows([]string{"id", "name", "position"}))
	mock.ExpectQuery("SELECT(.+)file(.+)").WillReturnRows(sqlmock.NewRows([]string{"id", "name", "dir"}))
	objects, err = fs.List(ctx, "/folder", func(s string) string {
		return "prefix" + s
	})
	asserts.Len(objects, 0)
	asserts.NoError(err)
	asserts.NoError(mock.ExpectationsWereMet())

	// 成功，子目錄路徑不存在
	mock.ExpectQuery("SELECT(.+)").
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "owner_id"}).AddRow(1, "/", 1))
	// folder
	mock.ExpectQuery("SELECT(.+)").
		WithArgs(1, 1, "folder").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "owner_id"}))

	objects, err = fs.List(ctx, "/folder", func(s string) string {
		return "prefix" + s
	})
	asserts.Len(objects, 0)
	asserts.NoError(mock.ExpectationsWereMet())
}

func TestFileSystem_CreateDirectory(t *testing.T) {
	asserts := assert.New(t)
	fs := &FileSystem{User: &model.User{
		Model: gorm.Model{
			ID: 1,
		},
	}}
	ctx := context.Background()

	// 目錄名非法
	_, err := fs.CreateDirectory(ctx, "/ad/a+?")
	asserts.Equal(ErrIllegalObjectName, err)

	// 存在同名文件
	// 根目錄
	mock.ExpectQuery("SELECT(.+)").
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "owner_id"}).AddRow(1, 1))
	// ad
	mock.ExpectQuery("SELECT(.+)").
		WithArgs(1, 1, "ad").
		WillReturnRows(sqlmock.NewRows([]string{"id", "owner_id"}).AddRow(2, 1))

	mock.ExpectQuery("SELECT(.+)files").WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(1, "ab"))
	_, err = fs.CreateDirectory(ctx, "/ad/ab")
	asserts.Equal(ErrFileExisted, err)
	asserts.NoError(mock.ExpectationsWereMet())

	// 存在同名目錄
	mock.ExpectQuery("SELECT(.+)").
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "owner_id"}).AddRow(1, 1))
	// ad
	mock.ExpectQuery("SELECT(.+)").
		WithArgs(1, 1, "ad").
		WillReturnRows(sqlmock.NewRows([]string{"id", "owner_id"}).AddRow(2, 1))

	mock.ExpectQuery("SELECT(.+)files").WillReturnRows(sqlmock.NewRows([]string{"id", "name"}))
	mock.ExpectBegin()
	mock.ExpectExec("INSERT(.+)").WillReturnError(errors.New("s"))
	mock.ExpectRollback()
	_, err = fs.CreateDirectory(ctx, "/ad/ab")
	asserts.Error(err)
	asserts.NoError(mock.ExpectationsWereMet())

	// 成功建立
	// 根目錄
	mock.ExpectQuery("SELECT(.+)").
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "owner_id"}).AddRow(1, 1))
	// ad
	mock.ExpectQuery("SELECT(.+)").
		WithArgs(1, 1, "ad").
		WillReturnRows(sqlmock.NewRows([]string{"id", "owner_id"}).AddRow(2, 1))

	mock.ExpectQuery("SELECT(.+)files").WillReturnRows(sqlmock.NewRows([]string{"id", "name"}))
	mock.ExpectBegin()
	mock.ExpectExec("INSERT(.+)").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	_, err = fs.CreateDirectory(ctx, "/ad/ab")
	asserts.NoError(err)
	asserts.NoError(mock.ExpectationsWereMet())

	// 父目錄不存在
	mock.ExpectQuery("SELECT(.+)folders").WillReturnRows(sqlmock.NewRows([]string{"id", "name"}))
	_, err = fs.CreateDirectory(ctx, "/ad")
	asserts.Equal(ErrRootProtected, err)
	asserts.NoError(mock.ExpectationsWereMet())
}

func TestFileSystem_ListDeleteFiles(t *testing.T) {
	conf.DatabaseConfig.Type = "mysql"
	asserts := assert.New(t)
	fs := &FileSystem{User: &model.User{
		Model: gorm.Model{
			ID: 1,
		},
	}}

	// 成功
	{
		mock.ExpectQuery("SELECT(.+)").WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(1, "1.txt").AddRow(2, "2.txt"))
		err := fs.ListDeleteFiles(context.Background(), []uint{1})
		asserts.NoError(err)
		asserts.NoError(mock.ExpectationsWereMet())
	}

	// 失敗
	{
		mock.ExpectQuery("SELECT(.+)").WillReturnError(errors.New("error"))
		err := fs.ListDeleteFiles(context.Background(), []uint{1})
		asserts.Error(err)
		asserts.Equal(serializer.CodeDBError, err.(serializer.AppError).Code)
		asserts.NoError(mock.ExpectationsWereMet())
	}
}

func TestFileSystem_ListDeleteDirs(t *testing.T) {
	conf.DatabaseConfig.Type = "mysql"
	asserts := assert.New(t)
	fs := &FileSystem{User: &model.User{
		Model: gorm.Model{
			ID: 1,
		},
	}}

	// 成功
	{
		mock.ExpectQuery("SELECT(.+)").
			WillReturnRows(
				sqlmock.NewRows([]string{"id"}).
					AddRow(1).
					AddRow(2).
					AddRow(3),
			)
		mock.ExpectQuery("SELECT(.+)").
			WithArgs(1, 2, 3).
			WillReturnRows(
				sqlmock.NewRows([]string{"id", "name"}).
					AddRow(4, "1.txt").
					AddRow(5, "2.txt").
					AddRow(6, "3.txt"),
			)
		err := fs.ListDeleteDirs(context.Background(), []uint{1})
		asserts.NoError(err)
		asserts.Len(fs.FileTarget, 3)
		asserts.Len(fs.DirTarget, 3)
		asserts.NoError(mock.ExpectationsWereMet())
	}

	// 檢索文件發生錯誤
	{
		mock.ExpectQuery("SELECT(.+)").
			WillReturnRows(
				sqlmock.NewRows([]string{"id"}).
					AddRow(1).
					AddRow(2).
					AddRow(3),
			)
		mock.ExpectQuery("SELECT(.+)").
			WithArgs(1, 2, 3).
			WillReturnError(errors.New("error"))
		err := fs.ListDeleteDirs(context.Background(), []uint{1})
		asserts.Error(err)
		asserts.Len(fs.DirTarget, 6)
		asserts.NoError(mock.ExpectationsWereMet())
	}
	// 檢索目錄發生錯誤
	{
		mock.ExpectQuery("SELECT(.+)").
			WillReturnError(errors.New("error"))
		err := fs.ListDeleteDirs(context.Background(), []uint{1})
		asserts.Error(err)
		asserts.NoError(mock.ExpectationsWereMet())
	}
}

func TestFileSystem_Delete(t *testing.T) {
	conf.DatabaseConfig.Type = "mysql"
	asserts := assert.New(t)
	cache.Set("pack_size_1", uint64(0), 0)
	fs := &FileSystem{User: &model.User{
		Model: gorm.Model{
			ID: 1,
		},
		Storage: 3,
		Group:   model.Group{MaxStorage: 3},
	}}
	ctx := context.Background()

	//全部未成功
	{
		// 列出要刪除的目錄
		mock.ExpectQuery("SELECT(.+)").
			WillReturnRows(
				sqlmock.NewRows([]string{"id"}).
					AddRow(1).
					AddRow(2).
					AddRow(3),
			)
		mock.ExpectQuery("SELECT(.+)").
			WithArgs(1, 2, 3).
			WillReturnRows(
				sqlmock.NewRows([]string{"id", "name", "source_name", "policy_id", "size"}).
					AddRow(4, "1.txt", "1.txt", 2, 1),
			)
		// 查詢頂級的文件
		mock.ExpectQuery("SELECT(.+)").WillReturnRows(sqlmock.NewRows([]string{"id", "name", "source_name", "policy_id", "size"}).AddRow(1, "1.txt", "1.txt", 603, 2))
		mock.ExpectQuery("SELECT(.+)files(.+)").
			WillReturnRows(sqlmock.NewRows([]string{"id", "policy_id", "source_name"}))
		// 尋找軟連接
		mock.ExpectQuery("SELECT(.+)").WillReturnRows(sqlmock.NewRows([]string{"id"}))
		// 查詢上傳策略
		mock.ExpectQuery("SELECT(.+)").WillReturnRows(sqlmock.NewRows([]string{"id", "type"}).AddRow(603, "local"))
		// 刪除文件記錄
		mock.ExpectBegin()
		mock.ExpectExec("DELETE(.+)files").
			WillReturnResult(sqlmock.NewResult(0, 3))
		mock.ExpectCommit()
		// 刪除對應分享
		mock.ExpectBegin()
		mock.ExpectExec("UPDATE(.+)shares").
			WillReturnResult(sqlmock.NewResult(0, 3))
		mock.ExpectCommit()

		err := fs.Delete(ctx, []uint{1}, []uint{1}, false)
		asserts.Error(err)
		asserts.Equal(203, err.(serializer.AppError).Code)
		asserts.Equal(uint64(3), fs.User.Storage)
		asserts.NoError(mock.ExpectationsWereMet())
	}
	//全部未成功，強制
	{
		fs.CleanTargets()
		mock.ExpectQuery("SELECT(.+)").
			WillReturnRows(
				sqlmock.NewRows([]string{"id"}).
					AddRow(1).
					AddRow(2).
					AddRow(3),
			)
		mock.ExpectQuery("SELECT(.+)").
			WithArgs(1, 2, 3).
			WillReturnRows(
				sqlmock.NewRows([]string{"id", "name", "source_name", "policy_id", "size"}).
					AddRow(4, "1.txt", "1.txt", 365, 1),
			)
		mock.ExpectQuery("SELECT(.+)").WillReturnRows(sqlmock.NewRows([]string{"id", "name", "source_name", "policy_id", "size"}).AddRow(1, "2.txt", "2.txt", 365, 2))
		mock.ExpectQuery("SELECT(.+)files(.+)").
			WillReturnRows(sqlmock.NewRows([]string{"id", "policy_id", "source_name"}))
		// 查詢上傳策略
		mock.ExpectQuery("SELECT(.+)").WillReturnRows(sqlmock.NewRows([]string{"id", "type"}).AddRow(365, "local"))
		// 刪除文件記錄
		mock.ExpectBegin()
		mock.ExpectExec("DELETE(.+)").
			WillReturnResult(sqlmock.NewResult(0, 3))
		mock.ExpectCommit()
		// 刪除對應分享
		mock.ExpectBegin()
		mock.ExpectExec("UPDATE(.+)shares").
			WillReturnResult(sqlmock.NewResult(0, 3))
		mock.ExpectCommit()
		// 歸還容量
		mock.ExpectBegin()
		mock.ExpectExec("UPDATE(.+)").
			WillReturnResult(sqlmock.NewResult(0, 3))
		mock.ExpectCommit()
		// 刪除目錄
		mock.ExpectBegin()
		mock.ExpectExec("DELETE(.+)").
			WillReturnResult(sqlmock.NewResult(0, 3))
		mock.ExpectCommit()
		// 刪除對應分享
		mock.ExpectBegin()
		mock.ExpectExec("UPDATE(.+)shares").
			WillReturnResult(sqlmock.NewResult(0, 3))
		mock.ExpectCommit()

		fs.FileTarget = []model.File{}
		fs.DirTarget = []model.Folder{}
		err := fs.Delete(ctx, []uint{1}, []uint{1}, true)
		asserts.NoError(err)
		asserts.Equal(uint64(0), fs.User.Storage)
	}
	//全部成功
	{
		fs.CleanTargets()
		file, err := os.Create(util.RelativePath("1.txt"))
		file2, err := os.Create(util.RelativePath("2.txt"))
		file.Close()
		file2.Close()
		asserts.NoError(err)
		mock.ExpectQuery("SELECT(.+)").
			WillReturnRows(
				sqlmock.NewRows([]string{"id"}).
					AddRow(1).
					AddRow(2).
					AddRow(3),
			)
		mock.ExpectQuery("SELECT(.+)").
			WithArgs(1, 2, 3).
			WillReturnRows(
				sqlmock.NewRows([]string{"id", "name", "source_name", "policy_id", "size"}).
					AddRow(4, "1.txt", "1.txt", 602, 1),
			)
		mock.ExpectQuery("SELECT(.+)").WillReturnRows(sqlmock.NewRows([]string{"id", "name", "source_name", "policy_id", "size"}).AddRow(1, "2.txt", "2.txt", 602, 2))
		mock.ExpectQuery("SELECT(.+)files(.+)").
			WillReturnRows(sqlmock.NewRows([]string{"id", "policy_id", "source_name"}))
		// 查詢上傳策略
		mock.ExpectQuery("SELECT(.+)").WillReturnRows(sqlmock.NewRows([]string{"id", "type"}).AddRow(602, "local"))
		// 刪除文件記錄
		mock.ExpectBegin()
		mock.ExpectExec("DELETE(.+)").
			WillReturnResult(sqlmock.NewResult(0, 3))
		mock.ExpectCommit()
		// 刪除對應分享
		mock.ExpectBegin()
		mock.ExpectExec("UPDATE(.+)shares").
			WillReturnResult(sqlmock.NewResult(0, 3))
		mock.ExpectCommit()
		// 歸還容量
		mock.ExpectBegin()
		mock.ExpectExec("UPDATE(.+)").
			WillReturnResult(sqlmock.NewResult(0, 3))
		mock.ExpectCommit()
		// 刪除目錄
		mock.ExpectBegin()
		mock.ExpectExec("DELETE(.+)").
			WillReturnResult(sqlmock.NewResult(0, 3))
		mock.ExpectCommit()
		// 刪除對應分享
		mock.ExpectBegin()
		mock.ExpectExec("UPDATE(.+)shares").
			WillReturnResult(sqlmock.NewResult(0, 3))
		mock.ExpectCommit()

		fs.FileTarget = []model.File{}
		fs.DirTarget = []model.Folder{}
		err = fs.Delete(ctx, []uint{1}, []uint{1}, false)
		asserts.NoError(err)
		asserts.Equal(uint64(0), fs.User.Storage)
	}

}

func TestFileSystem_Copy(t *testing.T) {
	asserts := assert.New(t)
	cache.Set("pack_size_1", uint64(0), 0)
	fs := &FileSystem{User: &model.User{
		Model: gorm.Model{
			ID: 1,
		},
		Storage: 3,
		Group:   model.Group{MaxStorage: 3},
	}}
	ctx := context.Background()

	// 目錄不存在
	{
		mock.ExpectQuery("SELECT(.+)").WillReturnRows(
			sqlmock.NewRows([]string{"name"}),
		)
		mock.ExpectQuery("SELECT(.+)").WillReturnRows(
			sqlmock.NewRows([]string{"name"}),
		)
		err := fs.Copy(ctx, []uint{}, []uint{}, "/src", "/dst")
		asserts.Equal(ErrPathNotExist, err)
		asserts.NoError(mock.ExpectationsWereMet())
	}

	// 複製目錄出錯
	{
		// 根目錄
		mock.ExpectQuery("SELECT(.+)").
			WithArgs(1).
			WillReturnRows(sqlmock.NewRows([]string{"id", "owner_id"}).AddRow(1, 1))
		// 1
		mock.ExpectQuery("SELECT(.+)").
			WithArgs(1, 1, "dst").
			WillReturnRows(sqlmock.NewRows([]string{"id", "owner_id"}).AddRow(2, 1))
		// 根目錄
		mock.ExpectQuery("SELECT(.+)").
			WithArgs(1).
			WillReturnRows(sqlmock.NewRows([]string{"id", "owner_id"}).AddRow(1, 1))
		// 1
		mock.ExpectQuery("SELECT(.+)").
			WithArgs(1, 1, "src").
			WillReturnRows(sqlmock.NewRows([]string{"id", "owner_id"}).AddRow(2, 1))

		err := fs.Copy(ctx, []uint{1}, []uint{}, "/src", "/dst")
		asserts.Error(err)
		asserts.NoError(mock.ExpectationsWereMet())
	}

}

func TestFileSystem_Move(t *testing.T) {
	asserts := assert.New(t)
	cache.Set("pack_size_1", uint64(0), 0)
	fs := &FileSystem{User: &model.User{
		Model: gorm.Model{
			ID: 1,
		},
		Storage: 3,
		Group:   model.Group{MaxStorage: 3},
	}}
	ctx := context.Background()

	// 目錄不存在
	{
		mock.ExpectQuery("SELECT(.+)").WillReturnRows(
			sqlmock.NewRows([]string{"name"}),
		)
		err := fs.Move(ctx, []uint{}, []uint{}, "/src", "/dst")
		asserts.Equal(ErrPathNotExist, err)
		asserts.NoError(mock.ExpectationsWereMet())
	}

	// 移動目錄出錯
	{
		// 根目錄
		mock.ExpectQuery("SELECT(.+)").
			WithArgs(1).
			WillReturnRows(sqlmock.NewRows([]string{"id", "owner_id"}).AddRow(1, 1))
		// 1
		mock.ExpectQuery("SELECT(.+)").
			WithArgs(1, 1, "dst").
			WillReturnRows(sqlmock.NewRows([]string{"id", "owner_id"}).AddRow(2, 1))
		// 根目錄
		mock.ExpectQuery("SELECT(.+)").
			WithArgs(1).
			WillReturnRows(sqlmock.NewRows([]string{"id", "owner_id"}).AddRow(1, 1))
		// 1
		mock.ExpectQuery("SELECT(.+)").
			WithArgs(1, 1, "src").
			WillReturnRows(sqlmock.NewRows([]string{"id", "owner_id"}).AddRow(2, 1))
		err := fs.Move(ctx, []uint{1}, []uint{}, "/src", "/dst")
		asserts.Error(err)
		asserts.NoError(mock.ExpectationsWereMet())
	}
}

func TestFileSystem_Rename(t *testing.T) {
	asserts := assert.New(t)
	fs := &FileSystem{User: &model.User{
		Model: gorm.Model{
			ID: 1,
		},
	}}
	ctx := context.Background()

	// 重新命名文件 成功
	{
		mock.ExpectQuery("SELECT(.+)files(.+)").
			WithArgs(10, 1).
			WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(10, "old.text"))
		mock.ExpectBegin()
		mock.ExpectExec("UPDATE(.+)files(.+)").
			WithArgs("new.txt", sqlmock.AnyArg(), 10).
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()
		err := fs.Rename(ctx, []uint{}, []uint{10}, "new.txt")
		asserts.NoError(mock.ExpectationsWereMet())
		asserts.NoError(err)
	}

	// 重新命名文件 不存在
	{
		mock.ExpectQuery("SELECT(.+)files(.+)").
			WithArgs(10, 1).
			WillReturnRows(sqlmock.NewRows([]string{"id", "name"}))
		err := fs.Rename(ctx, []uint{}, []uint{10}, "new.txt")
		asserts.NoError(mock.ExpectationsWereMet())
		asserts.Error(err)
		asserts.Equal(ErrPathNotExist, err)
	}

	// 重新命名文件 失敗
	{
		mock.ExpectQuery("SELECT(.+)files(.+)").
			WithArgs(10, 1).
			WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(10, "old.text"))
		mock.ExpectBegin()
		mock.ExpectExec("UPDATE(.+)files(.+)").
			WithArgs("new.txt", sqlmock.AnyArg(), 10).
			WillReturnError(errors.New("error"))
		mock.ExpectRollback()
		err := fs.Rename(ctx, []uint{}, []uint{10}, "new.txt")
		asserts.NoError(mock.ExpectationsWereMet())
		asserts.Error(err)
		asserts.Equal(ErrFileExisted, err)
	}

	// 重新命名目錄 成功
	{
		mock.ExpectQuery("SELECT(.+)folders(.+)").
			WithArgs(10, 1).
			WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(10, "old"))
		mock.ExpectBegin()
		mock.ExpectExec("UPDATE(.+)folders(.+)").
			WithArgs("new", sqlmock.AnyArg(), 10).
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()
		err := fs.Rename(ctx, []uint{10}, []uint{}, "new")
		asserts.NoError(mock.ExpectationsWereMet())
		asserts.NoError(err)
	}

	// 重新命名目錄 不存在
	{
		mock.ExpectQuery("SELECT(.+)folders(.+)").
			WithArgs(10, 1).
			WillReturnRows(sqlmock.NewRows([]string{"id", "name"}))
		err := fs.Rename(ctx, []uint{10}, []uint{}, "new")
		asserts.NoError(mock.ExpectationsWereMet())
		asserts.Error(err)
		asserts.Equal(ErrPathNotExist, err)
	}

	// 重新命名目錄 失敗
	{
		mock.ExpectQuery("SELECT(.+)folders(.+)").
			WithArgs(10, 1).
			WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(10, "old"))
		mock.ExpectBegin()
		mock.ExpectExec("UPDATE(.+)folders(.+)").
			WithArgs("new", sqlmock.AnyArg(), 10).
			WillReturnError(errors.New("error"))
		mock.ExpectRollback()
		err := fs.Rename(ctx, []uint{10}, []uint{}, "new")
		asserts.NoError(mock.ExpectationsWereMet())
		asserts.Error(err)
		asserts.Equal(ErrFileExisted, err)
	}

	// 未選中任何物件
	{
		err := fs.Rename(ctx, []uint{}, []uint{}, "new")
		asserts.Error(err)
		asserts.Equal(ErrPathNotExist, err)
	}

	// 新名字是目錄，不合法
	{
		err := fs.Rename(ctx, []uint{10}, []uint{}, "ne/w")
		asserts.Error(err)
		asserts.Equal(ErrIllegalObjectName, err)
	}

	// 新名字是文件，不合法
	{
		err := fs.Rename(ctx, []uint{}, []uint{10}, "ne/w")
		asserts.Error(err)
		asserts.Equal(ErrIllegalObjectName, err)
	}

	// 新名字是文件，副檔名不合法
	{
		fs.User.Policy.OptionsSerialized.FileType = []string{"txt"}
		err := fs.Rename(ctx, []uint{}, []uint{10}, "1.jpg")
		asserts.Error(err)
		asserts.Equal(ErrIllegalObjectName, err)
	}

	// 新名字是目錄，不應該檢測副檔名
	{
		fs.User.Policy.OptionsSerialized.FileType = []string{"txt"}
		mock.ExpectQuery("SELECT(.+)folders(.+)").
			WithArgs(10, 1).
			WillReturnRows(sqlmock.NewRows([]string{"id", "name"}))
		err := fs.Rename(ctx, []uint{10}, []uint{}, "new")
		asserts.NoError(mock.ExpectationsWereMet())
		asserts.Error(err)
		asserts.Equal(ErrPathNotExist, err)
	}
}

func TestFileSystem_SaveTo(t *testing.T) {
	asserts := assert.New(t)
	fs := &FileSystem{User: &model.User{
		Model: gorm.Model{
			ID: 1,
		},
	}}
	ctx := context.Background()

	// 單文件 失敗
	{
		// 根目錄
		mock.ExpectQuery("SELECT(.+)").
			WithArgs(1).
			WillReturnRows(sqlmock.NewRows([]string{"id", "owner_id"}).AddRow(1, 1))
		mock.ExpectQuery("SELECT(.+)").WillReturnError(errors.New("error"))
		fs.SetTargetFile(&[]model.File{{Name: "test.txt"}})
		err := fs.SaveTo(ctx, "/")
		asserts.NoError(mock.ExpectationsWereMet())
		asserts.Error(err)
	}
	// 目錄 成功
	{
		// 根目錄
		mock.ExpectQuery("SELECT(.+)").
			WithArgs(1).
			WillReturnRows(sqlmock.NewRows([]string{"id", "owner_id"}).AddRow(1, 1))
		mock.ExpectQuery("SELECT(.+)").WillReturnError(errors.New("error"))
		fs.SetTargetDir(&[]model.Folder{{Name: "folder"}})
		err := fs.SaveTo(ctx, "/")
		asserts.NoError(mock.ExpectationsWereMet())
		asserts.Error(err)
	}
	// 父目錄不存在
	{
		// 根目錄
		mock.ExpectQuery("SELECT(.+)").
			WithArgs(1).
			WillReturnRows(sqlmock.NewRows([]string{"id", "owner_id"}))
		fs.SetTargetDir(&[]model.Folder{{Name: "folder"}})
		err := fs.SaveTo(ctx, "/")
		asserts.NoError(mock.ExpectationsWereMet())
		asserts.Error(err)
	}
}
