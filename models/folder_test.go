package model

import (
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/cloudreve/Cloudreve/v3/pkg/conf"
	"github.com/jinzhu/gorm"
	"github.com/stretchr/testify/assert"
)

func TestFolder_Create(t *testing.T) {
	asserts := assert.New(t)
	folder := &Folder{
		Name: "new folder",
	}

	// 插入成功
	mock.ExpectBegin()
	mock.ExpectExec("INSERT(.+)").WillReturnResult(sqlmock.NewResult(5, 1))
	mock.ExpectCommit()
	fid, err := folder.Create()
	asserts.NoError(err)
	asserts.Equal(uint(5), fid)
	asserts.NoError(mock.ExpectationsWereMet())

	// 插入失敗
	mock.ExpectBegin()
	mock.ExpectExec("INSERT(.+)").WillReturnError(errors.New("error"))
	mock.ExpectRollback()
	fid, err = folder.Create()
	asserts.Error(err)
	asserts.Equal(uint(0), fid)
	asserts.NoError(mock.ExpectationsWereMet())
}

func TestFolder_GetChild(t *testing.T) {
	asserts := assert.New(t)
	folder := Folder{
		Model:   gorm.Model{ID: 5},
		OwnerID: 1,
		Name:    "/",
	}

	// 目錄存在
	{
		mock.ExpectQuery("SELECT(.+)").
			WithArgs(5, 1, "sub").
			WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(1, "sub"))
		sub, err := folder.GetChild("sub")
		asserts.NoError(mock.ExpectationsWereMet())
		asserts.NoError(err)
		asserts.Equal(sub.Name, "sub")
		asserts.Equal("/", sub.Position)
	}

	// 目錄不存在
	{
		mock.ExpectQuery("SELECT(.+)").
			WithArgs(5, 1, "sub").
			WillReturnRows(sqlmock.NewRows([]string{"id", "name"}))
		sub, err := folder.GetChild("sub")
		asserts.NoError(mock.ExpectationsWereMet())
		asserts.Error(err)
		asserts.Equal(uint(0), sub.ID)

	}
}

func TestFolder_GetChildFolder(t *testing.T) {
	asserts := assert.New(t)
	folder := &Folder{
		Model: gorm.Model{
			ID: 1,
		},
		Position: "/123",
		Name:     "456",
	}

	// 找不到
	mock.ExpectQuery("SELECT(.+)parent_id(.+)").WithArgs(1).WillReturnError(errors.New("error"))
	files, err := folder.GetChildFolder()
	asserts.Error(err)
	asserts.Len(files, 0)
	asserts.NoError(mock.ExpectationsWereMet())

	// 找到了
	mock.ExpectQuery("SELECT(.+)parent_id(.+)").WithArgs(1).WillReturnRows(sqlmock.NewRows([]string{"name", "id"}).AddRow("1.txt", 1).AddRow("2.txt", 2))
	files, err = folder.GetChildFolder()
	asserts.NoError(err)
	asserts.Len(files, 2)
	asserts.Equal("/123/456", files[0].Position)
	asserts.NoError(mock.ExpectationsWereMet())
}

func TestGetRecursiveChildFolderSQLite(t *testing.T) {
	conf.DatabaseConfig.Type = "sqlite3"
	asserts := assert.New(t)

	// 測試目錄結構
	//      1
	//     2  3
	//   4  5   6

	// 查詢第一層
	mock.ExpectQuery("SELECT(.+)").
		WithArgs(1, 1).
		WillReturnRows(
			sqlmock.NewRows([]string{"id", "name"}).
				AddRow(1, "folder1"),
		)
	// 查詢第二層
	mock.ExpectQuery("SELECT(.+)").
		WithArgs(1, 1).
		WillReturnRows(
			sqlmock.NewRows([]string{"id", "name"}).
				AddRow(2, "folder2").
				AddRow(3, "folder3"),
		)
	// 查詢第三層
	mock.ExpectQuery("SELECT(.+)").
		WithArgs(1, 2, 3).
		WillReturnRows(
			sqlmock.NewRows([]string{"id", "name"}).
				AddRow(4, "folder4").
				AddRow(5, "folder5").
				AddRow(6, "folder6"),
		)
	// 查詢第四層
	mock.ExpectQuery("SELECT(.+)").
		WithArgs(1, 4, 5, 6).
		WillReturnRows(
			sqlmock.NewRows([]string{"id", "name"}),
		)

	folders, err := GetRecursiveChildFolder([]uint{1}, 1, true)
	asserts.NoError(err)
	asserts.NoError(mock.ExpectationsWereMet())
	asserts.Len(folders, 6)
}

func TestDeleteFolderByIDs(t *testing.T) {
	asserts := assert.New(t)

	// 出錯
	{
		mock.ExpectBegin()
		mock.ExpectExec("DELETE(.+)").
			WillReturnError(errors.New("error"))
		mock.ExpectRollback()
		err := DeleteFolderByIDs([]uint{1, 2, 3})
		asserts.NoError(mock.ExpectationsWereMet())
		asserts.Error(err)
	}
	// 成功
	{
		mock.ExpectBegin()
		mock.ExpectExec("DELETE(.+)").
			WillReturnResult(sqlmock.NewResult(0, 3))
		mock.ExpectCommit()
		err := DeleteFolderByIDs([]uint{1, 2, 3})
		asserts.NoError(mock.ExpectationsWereMet())
		asserts.NoError(err)
	}
}

func TestGetFoldersByIDs(t *testing.T) {
	asserts := assert.New(t)

	// 出錯
	{
		mock.ExpectQuery("SELECT(.+)").
			WithArgs(1, 2, 3, 1).
			WillReturnError(errors.New("error"))
		folders, err := GetFoldersByIDs([]uint{1, 2, 3}, 1)
		asserts.NoError(mock.ExpectationsWereMet())
		asserts.Error(err)
		asserts.Len(folders, 0)
	}

	// 部分找到
	{
		mock.ExpectQuery("SELECT(.+)").
			WithArgs(1, 2, 3, 1).
			WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(1, "1"))
		folders, err := GetFoldersByIDs([]uint{1, 2, 3}, 1)
		asserts.NoError(mock.ExpectationsWereMet())
		asserts.NoError(err)
		asserts.Len(folders, 1)
	}
}

func TestFolder_MoveOrCopyFileTo(t *testing.T) {
	asserts := assert.New(t)
	// 目前目錄
	folder := Folder{
		Model:   gorm.Model{ID: 1},
		OwnerID: 1,
		Name:    "test",
	}
	// 目標目錄
	dstFolder := Folder{
		Model: gorm.Model{ID: 10},
		Name:  "dst",
	}

	// 複製文件
	{
		mock.ExpectQuery("SELECT(.+)").
			WithArgs(
				1,
				2,
				1,
				1,
			).WillReturnRows(
			sqlmock.NewRows([]string{"id", "size"}).
				AddRow(1, 10).
				AddRow(2, 20),
		)
		mock.ExpectBegin()
		mock.ExpectExec("INSERT(.+)").WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()
		mock.ExpectBegin()
		mock.ExpectExec("INSERT(.+)").WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()
		storage, err := folder.MoveOrCopyFileTo(
			[]uint{1, 2},
			&dstFolder,
			true,
		)
		asserts.NoError(err)
		asserts.NoError(mock.ExpectationsWereMet())
		asserts.Equal(uint64(30), storage)
	}

	// 複製文件, 檢索文件出錯
	{
		mock.ExpectQuery("SELECT(.+)").
			WithArgs(
				1,
				2,
				1,
				1,
			).WillReturnError(errors.New("error"))

		storage, err := folder.MoveOrCopyFileTo(
			[]uint{1, 2},
			&dstFolder,
			true,
		)
		asserts.Error(err)
		asserts.NoError(mock.ExpectationsWereMet())
		asserts.Equal(uint64(0), storage)
	}

	// 複製文件,第二個文件插入出錯
	{
		mock.ExpectQuery("SELECT(.+)").
			WithArgs(
				1,
				2,
				1,
				1,
			).WillReturnRows(
			sqlmock.NewRows([]string{"id", "size"}).
				AddRow(1, 10).
				AddRow(2, 20),
		)
		mock.ExpectBegin()
		mock.ExpectExec("INSERT(.+)").WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()
		mock.ExpectBegin()
		mock.ExpectExec("INSERT(.+)").WillReturnError(errors.New("error"))
		mock.ExpectRollback()
		storage, err := folder.MoveOrCopyFileTo(
			[]uint{1, 2},
			&dstFolder,
			true,
		)
		asserts.Error(err)
		asserts.NoError(mock.ExpectationsWereMet())
		asserts.Equal(uint64(10), storage)
	}

	// 移動文件 成功
	{
		mock.ExpectBegin()
		mock.ExpectExec("UPDATE(.+)").
			WithArgs(10, sqlmock.AnyArg(), 1, 2, 1, 1).
			WillReturnResult(sqlmock.NewResult(1, 2))
		mock.ExpectCommit()
		storage, err := folder.MoveOrCopyFileTo(
			[]uint{1, 2},
			&dstFolder,
			false,
		)
		asserts.NoError(mock.ExpectationsWereMet())
		asserts.NoError(err)
		asserts.Equal(uint64(0), storage)
	}

	// 移動文件 出錯
	{
		mock.ExpectBegin()
		mock.ExpectExec("UPDATE(.+)").
			WithArgs(10, sqlmock.AnyArg(), 1, 2, 1, 1).
			WillReturnError(errors.New("error"))
		mock.ExpectRollback()
		storage, err := folder.MoveOrCopyFileTo(
			[]uint{1, 2},
			&dstFolder,
			false,
		)
		asserts.NoError(mock.ExpectationsWereMet())
		asserts.Error(err)
		asserts.Equal(uint64(0), storage)
	}
}

func TestFolder_CopyFolderTo(t *testing.T) {
	conf.DatabaseConfig.Type = "mysql"
	asserts := assert.New(t)
	// 父目錄
	parFolder := Folder{
		Model:   gorm.Model{ID: 9},
		OwnerID: 1,
	}
	// 目標目錄
	dstFolder := Folder{
		Model: gorm.Model{ID: 10},
	}

	// 測試複製目錄結構
	//       test(2)(5)
	//    1(3)(6)    2.txt
	//  3(4)(7) 4.txt

	// 正常情況 成功
	{
		// GetRecursiveChildFolder
		mock.ExpectQuery("SELECT(.+)").WithArgs(1, 2).WillReturnRows(sqlmock.NewRows([]string{"id", "parent_id"}).AddRow(2, 9))
		mock.ExpectQuery("SELECT(.+)").WithArgs(1, 2).WillReturnRows(sqlmock.NewRows([]string{"id", "parent_id"}).AddRow(3, 2))
		mock.ExpectQuery("SELECT(.+)").WithArgs(1, 3).WillReturnRows(sqlmock.NewRows([]string{"id", "parent_id"}).AddRow(4, 3))
		mock.ExpectQuery("SELECT(.+)").WithArgs(1, 4).WillReturnRows(sqlmock.NewRows([]string{"id", "parent_id"}))

		// 複製目錄
		mock.ExpectBegin()
		mock.ExpectExec("INSERT(.+)").WillReturnResult(sqlmock.NewResult(5, 1))
		mock.ExpectCommit()
		mock.ExpectBegin()
		mock.ExpectExec("INSERT(.+)").WillReturnResult(sqlmock.NewResult(6, 1))
		mock.ExpectCommit()
		mock.ExpectBegin()
		mock.ExpectExec("INSERT(.+)").WillReturnResult(sqlmock.NewResult(7, 1))
		mock.ExpectCommit()

		// 尋找子文件
		mock.ExpectQuery("SELECT(.+)").
			WithArgs(1, 2, 3, 4).
			WillReturnRows(
				sqlmock.NewRows([]string{"id", "name", "folder_id", "size"}).
					AddRow(1, "2.txt", 2, 10).
					AddRow(2, "3.txt", 3, 20),
			)

		// 複製子文件
		mock.ExpectBegin()
		mock.ExpectExec("INSERT(.+)").WillReturnResult(sqlmock.NewResult(5, 1))
		mock.ExpectCommit()
		mock.ExpectBegin()
		mock.ExpectExec("INSERT(.+)").WillReturnResult(sqlmock.NewResult(6, 1))
		mock.ExpectCommit()

		size, err := parFolder.CopyFolderTo(2, &dstFolder)
		asserts.NoError(mock.ExpectationsWereMet())
		asserts.NoError(err)
		asserts.Equal(uint64(30), size)
	}

	// 遞迴查詢失敗
	{
		// GetRecursiveChildFolder
		mock.ExpectQuery("SELECT(.+)").WithArgs(1, 2).WillReturnError(errors.New("error"))

		size, err := parFolder.CopyFolderTo(2, &dstFolder)
		asserts.NoError(mock.ExpectationsWereMet())
		asserts.Error(err)
		asserts.Equal(uint64(0), size)
	}

	// 父目錄ID不存在
	{
		// GetRecursiveChildFolder
		mock.ExpectQuery("SELECT(.+)").WithArgs(1, 2).WillReturnRows(sqlmock.NewRows([]string{"id", "parent_id"}).AddRow(2, 9))
		mock.ExpectQuery("SELECT(.+)").WithArgs(1, 2).WillReturnRows(sqlmock.NewRows([]string{"id", "parent_id"}).AddRow(3, 99))
		mock.ExpectQuery("SELECT(.+)").WithArgs(1, 3).WillReturnRows(sqlmock.NewRows([]string{"id", "parent_id"}).AddRow(4, 3))
		mock.ExpectQuery("SELECT(.+)").WithArgs(1, 4).WillReturnRows(sqlmock.NewRows([]string{"id", "parent_id"}))

		// 複製目錄
		mock.ExpectBegin()
		mock.ExpectExec("INSERT(.+)").WillReturnResult(sqlmock.NewResult(5, 1))
		mock.ExpectCommit()

		size, err := parFolder.CopyFolderTo(2, &dstFolder)
		asserts.NoError(mock.ExpectationsWereMet())
		asserts.Error(err)
		asserts.Equal(uint64(0), size)
	}

	// 查詢子文件失敗
	{
		// GetRecursiveChildFolder
		mock.ExpectQuery("SELECT(.+)").WithArgs(1, 2).WillReturnRows(sqlmock.NewRows([]string{"id", "parent_id"}).AddRow(2, 9))
		mock.ExpectQuery("SELECT(.+)").WithArgs(1, 2).WillReturnRows(sqlmock.NewRows([]string{"id", "parent_id"}).AddRow(3, 2))
		mock.ExpectQuery("SELECT(.+)").WithArgs(1, 3).WillReturnRows(sqlmock.NewRows([]string{"id", "parent_id"}).AddRow(4, 3))
		mock.ExpectQuery("SELECT(.+)").WithArgs(1, 4).WillReturnRows(sqlmock.NewRows([]string{"id", "parent_id"}))

		// 複製目錄
		mock.ExpectBegin()
		mock.ExpectExec("INSERT(.+)").WillReturnResult(sqlmock.NewResult(5, 1))
		mock.ExpectCommit()
		mock.ExpectBegin()
		mock.ExpectExec("INSERT(.+)").WillReturnResult(sqlmock.NewResult(6, 1))
		mock.ExpectCommit()
		mock.ExpectBegin()
		mock.ExpectExec("INSERT(.+)").WillReturnResult(sqlmock.NewResult(7, 1))
		mock.ExpectCommit()

		// 尋找子文件
		mock.ExpectQuery("SELECT(.+)").
			WithArgs(1, 2, 3, 4).
			WillReturnError(errors.New("error"))

		size, err := parFolder.CopyFolderTo(2, &dstFolder)
		asserts.NoError(mock.ExpectationsWereMet())
		asserts.Error(err)
		asserts.Equal(uint64(0), size)
	}

	// 複製文件  一個失敗
	{
		// GetRecursiveChildFolder
		mock.ExpectQuery("SELECT(.+)").WithArgs(1, 2).WillReturnRows(sqlmock.NewRows([]string{"id", "parent_id"}).AddRow(2, 9))
		mock.ExpectQuery("SELECT(.+)").WithArgs(1, 2).WillReturnRows(sqlmock.NewRows([]string{"id", "parent_id"}).AddRow(3, 2))
		mock.ExpectQuery("SELECT(.+)").WithArgs(1, 3).WillReturnRows(sqlmock.NewRows([]string{"id", "parent_id"}).AddRow(4, 3))
		mock.ExpectQuery("SELECT(.+)").WithArgs(1, 4).WillReturnRows(sqlmock.NewRows([]string{"id", "parent_id"}))

		// 複製目錄
		mock.ExpectBegin()
		mock.ExpectExec("INSERT(.+)").WillReturnResult(sqlmock.NewResult(5, 1))
		mock.ExpectCommit()
		mock.ExpectBegin()
		mock.ExpectExec("INSERT(.+)").WillReturnResult(sqlmock.NewResult(6, 1))
		mock.ExpectCommit()
		mock.ExpectBegin()
		mock.ExpectExec("INSERT(.+)").WillReturnResult(sqlmock.NewResult(7, 1))
		mock.ExpectCommit()

		// 尋找子文件
		mock.ExpectQuery("SELECT(.+)").
			WithArgs(1, 2, 3, 4).
			WillReturnRows(
				sqlmock.NewRows([]string{"id", "name", "folder_id", "size"}).
					AddRow(1, "2.txt", 2, 10).
					AddRow(2, "3.txt", 3, 20),
			)

		// 複製子文件
		mock.ExpectBegin()
		mock.ExpectExec("INSERT(.+)").WillReturnResult(sqlmock.NewResult(5, 1))
		mock.ExpectCommit()
		mock.ExpectBegin()
		mock.ExpectExec("INSERT(.+)").WillReturnError(errors.New("error"))
		mock.ExpectRollback()

		size, err := parFolder.CopyFolderTo(2, &dstFolder)
		asserts.NoError(mock.ExpectationsWereMet())
		asserts.Error(err)
		asserts.Equal(uint64(10), size)
	}

}

func TestFolder_MoveOrCopyFolderTo_Move(t *testing.T) {
	conf.DatabaseConfig.Type = "mysql"
	asserts := assert.New(t)
	// 父目錄
	parFolder := Folder{
		Model:   gorm.Model{ID: 9},
		OwnerID: 1,
	}
	// 目標目錄
	dstFolder := Folder{
		Model: gorm.Model{ID: 10},
	}

	// 成功
	{
		mock.ExpectBegin()
		mock.ExpectExec("UPDATE(.+)").
			WithArgs(10, sqlmock.AnyArg(), 1, 2, 1, 9).
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()
		err := parFolder.MoveFolderTo([]uint{1, 2}, &dstFolder)
		asserts.NoError(mock.ExpectationsWereMet())
		asserts.NoError(err)
	}
}

func TestFolder_FileInfoInterface(t *testing.T) {
	asserts := assert.New(t)
	folder := Folder{
		Model: gorm.Model{
			UpdatedAt: time.Date(2019, 12, 21, 12, 40, 0, 0, time.UTC),
		},
		Name:     "test_name",
		OwnerID:  0,
		Position: "/test",
	}

	name := folder.GetName()
	asserts.Equal("test_name", name)

	size := folder.GetSize()
	asserts.Equal(uint64(0), size)

	asserts.Equal(time.Date(2019, 12, 21, 12, 40, 0, 0, time.UTC), folder.ModTime())
	asserts.True(folder.IsDir())
	asserts.Equal("/test", folder.GetPosition())
}

func TestTraceRoot(t *testing.T) {
	asserts := assert.New(t)
	var parentId uint
	parentId = 5
	folder := Folder{
		ParentID: &parentId,
		OwnerID:  1,
		Name:     "test_name",
	}

	// 成功
	{
		mock.ExpectQuery("SELECT(.+)").WithArgs(5, 1).
			WillReturnRows(sqlmock.NewRows([]string{"id", "name", "parent_id"}).AddRow(5, "parent", 1))
		mock.ExpectQuery("SELECT(.+)").WithArgs(1, 0).
			WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(5, "/"))
		asserts.NoError(folder.TraceRoot())
		asserts.Equal("/parent", folder.Position)
		asserts.NoError(mock.ExpectationsWereMet())
	}

	// 出現錯誤
	// 成功
	{
		mock.ExpectQuery("SELECT(.+)").WithArgs(5, 1).
			WillReturnRows(sqlmock.NewRows([]string{"id", "name", "parent_id"}).AddRow(5, "parent", 1))
		mock.ExpectQuery("SELECT(.+)").WithArgs(1, 0).
			WillReturnError(errors.New("error"))
		asserts.Error(folder.TraceRoot())
		asserts.Equal("parent", folder.Position)
		asserts.NoError(mock.ExpectationsWereMet())
	}
}
