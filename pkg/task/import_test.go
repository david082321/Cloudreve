package task

import (
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	model "github.com/cloudreve/Cloudreve/v3/models"
	"github.com/cloudreve/Cloudreve/v3/pkg/cache"
	"github.com/cloudreve/Cloudreve/v3/pkg/util"
	"github.com/jinzhu/gorm"
	"github.com/stretchr/testify/assert"
)

func TestImportTask_Props(t *testing.T) {
	asserts := assert.New(t)
	task := &ImportTask{
		User: &model.User{},
	}
	asserts.NotEmpty(task.Props())
	asserts.Equal(ImportTaskType, task.Type())
	asserts.EqualValues(0, task.Creator())
	asserts.Nil(task.Model())
}

func TestImportTask_SetStatus(t *testing.T) {
	asserts := assert.New(t)
	task := &ImportTask{
		User: &model.User{},
		TaskModel: &model.Task{
			Model: gorm.Model{ID: 1},
		},
	}
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE(.+)").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	task.SetStatus(3)
	asserts.NoError(mock.ExpectationsWereMet())
}

func TestImportTask_SetError(t *testing.T) {
	asserts := assert.New(t)
	task := &ImportTask{
		User: &model.User{},
		TaskModel: &model.Task{
			Model: gorm.Model{ID: 1},
		},
	}

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE(.+)").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	task.SetErrorMsg("error", nil)
	asserts.NoError(mock.ExpectationsWereMet())
	asserts.Equal("error", task.GetError().Msg)
}

func TestImportTask_Do(t *testing.T) {
	asserts := assert.New(t)
	task := &ImportTask{
		User: &model.User{},
		TaskModel: &model.Task{
			Model: gorm.Model{ID: 1},
		},
		TaskProps: ImportProps{
			PolicyID:  63,
			Src:       "",
			Recursive: false,
			Dst:       "",
		},
	}

	// 儲存策略不存在
	{
		cache.Deletes([]string{"63"}, "policy_")
		mock.ExpectQuery("SELECT(.+)policies(.+)").
			WillReturnRows(sqlmock.NewRows([]string{"id"}))
		// 設定失敗狀態
		mock.ExpectBegin()
		mock.ExpectExec("UPDATE(.+)").WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()
		task.Do()
		asserts.NoError(mock.ExpectationsWereMet())
		asserts.NotEmpty(task.Err.Error)
		task.Err = nil
	}

	// 無法分配 Filesystem
	{
		cache.Deletes([]string{"63"}, "policy_")
		mock.ExpectQuery("SELECT(.+)policies(.+)").
			WillReturnRows(sqlmock.NewRows([]string{"id", "type"}).AddRow(63, "unknown"))
		// 設定失敗狀態
		mock.ExpectBegin()
		mock.ExpectExec("UPDATE(.+)").WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()
		task.Do()
		asserts.NoError(mock.ExpectationsWereMet())
		asserts.NotEmpty(task.Err.Msg)
		task.Err = nil
	}

	// 成功列取，但是文件為空
	{
		cache.Deletes([]string{"63"}, "policy_")
		task.TaskProps.Src = "TestImportTask_Do/empty"
		mock.ExpectQuery("SELECT(.+)policies(.+)").
			WillReturnRows(sqlmock.NewRows([]string{"id", "type"}).AddRow(63, "local"))
		// 設定listing狀態
		mock.ExpectBegin()
		mock.ExpectExec("UPDATE(.+)").WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()
		// 設定inserting狀態
		mock.ExpectBegin()
		mock.ExpectExec("UPDATE(.+)").WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()
		task.Do()
		asserts.NoError(mock.ExpectationsWereMet())
		asserts.Nil(task.Err)
		task.Err = nil
	}

	// 建立測試文件
	f, _ := util.CreatNestedFile(util.RelativePath("tests/TestImportTask_Do/test.txt"))
	f.Close()

	// 成功列取，包含一個文件一個目錄,父目錄建立失敗
	{
		cache.Deletes([]string{"63"}, "policy_")
		task.TaskProps.Src = "tests"
		task.TaskProps.Dst = "/"
		task.TaskProps.Recursive = true
		mock.ExpectQuery("SELECT(.+)policies(.+)").
			WillReturnRows(sqlmock.NewRows([]string{"id", "type"}).AddRow(63, "local"))
		// 設定listing狀態
		mock.ExpectBegin()
		mock.ExpectExec("UPDATE(.+)").WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()
		// 設定inserting狀態
		mock.ExpectBegin()
		mock.ExpectExec("UPDATE(.+)").WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()
		// 尋找父目錄，但是不存在
		mock.ExpectQuery("SELECT(.+)folders").WillReturnRows(sqlmock.NewRows([]string{"id"}))
		// 仍然不存在
		mock.ExpectQuery("SELECT(.+)folders").WillReturnRows(sqlmock.NewRows([]string{"id"}))
		// 建立文件時尋找父目錄，仍然不存在
		mock.ExpectQuery("SELECT(.+)folders").WillReturnRows(sqlmock.NewRows([]string{"id"}))

		task.Do()

		asserts.NoError(mock.ExpectationsWereMet())
		asserts.Nil(task.Err)
		task.Err = nil
	}

	// 成功列取，包含一個文件一個目錄, 全部操作成功
	{
		cache.Deletes([]string{"63"}, "policy_")
		task.TaskProps.Src = "tests"
		task.TaskProps.Dst = "/"
		task.TaskProps.Recursive = true
		mock.ExpectQuery("SELECT(.+)policies(.+)").
			WillReturnRows(sqlmock.NewRows([]string{"id", "type"}).AddRow(63, "local"))
		// 設定listing狀態
		mock.ExpectBegin()
		mock.ExpectExec("UPDATE(.+)").WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()
		// 設定inserting狀態
		mock.ExpectBegin()
		mock.ExpectExec("UPDATE(.+)").WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()
		// 尋找父目錄，存在
		mock.ExpectQuery("SELECT(.+)folders").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
		// 尋找同名文件，不存在
		mock.ExpectQuery("SELECT(.+)files").WillReturnRows(sqlmock.NewRows([]string{"id"}))
		// 建立目錄
		mock.ExpectBegin()
		mock.ExpectExec("INSERT(.+)folders(.+)").WillReturnResult(sqlmock.NewResult(2, 1))
		mock.ExpectCommit()
		// 插入文件記錄
		mock.ExpectBegin()
		mock.ExpectExec("INSERT(.+)files(.+)").WillReturnResult(sqlmock.NewResult(2, 1))
		mock.ExpectCommit()

		task.Do()

		asserts.NoError(mock.ExpectationsWereMet())
		asserts.Nil(task.Err)
		task.Err = nil
	}
}

func TestNewImportTask(t *testing.T) {
	asserts := assert.New(t)

	// 成功
	{
		mock.ExpectQuery("SELECT(.+)").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
		mock.ExpectBegin()
		mock.ExpectExec("INSERT(.+)").WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()
		job, err := NewImportTask(1, 1, "/", "/", false)
		asserts.NoError(mock.ExpectationsWereMet())
		asserts.NotNil(job)
		asserts.NoError(err)
	}

	// 失敗
	{
		mock.ExpectQuery("SELECT(.+)").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
		mock.ExpectBegin()
		mock.ExpectExec("INSERT(.+)").WillReturnError(errors.New("error"))
		mock.ExpectRollback()
		job, err := NewImportTask(1, 1, "/", "/", false)
		asserts.NoError(mock.ExpectationsWereMet())
		asserts.Nil(job)
		asserts.Error(err)
	}
}

func TestNewImportTaskFromModel(t *testing.T) {
	asserts := assert.New(t)

	// 成功
	{
		mock.ExpectQuery("SELECT(.+)").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
		job, err := NewImportTaskFromModel(&model.Task{Props: "{}"})
		asserts.NoError(mock.ExpectationsWereMet())
		asserts.NoError(err)
		asserts.NotNil(job)
	}

	// JSON解析失敗
	{
		mock.ExpectQuery("SELECT(.+)").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
		job, err := NewImportTaskFromModel(&model.Task{Props: "?"})
		asserts.NoError(mock.ExpectationsWereMet())
		asserts.Error(err)
		asserts.Nil(job)
	}
}
