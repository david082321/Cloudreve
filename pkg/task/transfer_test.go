package task

import (
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	model "github.com/cloudreve/Cloudreve/v3/models"
	"github.com/jinzhu/gorm"
	"github.com/stretchr/testify/assert"
)

func TestTransferTask_Props(t *testing.T) {
	asserts := assert.New(t)
	task := &TransferTask{
		User: &model.User{},
	}
	asserts.NotEmpty(task.Props())
	asserts.Equal(TransferTaskType, task.Type())
	asserts.EqualValues(0, task.Creator())
	asserts.Nil(task.Model())
}

func TestTransferTask_SetStatus(t *testing.T) {
	asserts := assert.New(t)
	task := &TransferTask{
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

func TestTransferTask_SetError(t *testing.T) {
	asserts := assert.New(t)
	task := &TransferTask{
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

func TestTransferTask_Do(t *testing.T) {
	asserts := assert.New(t)
	task := &TransferTask{
		TaskModel: &model.Task{
			Model: gorm.Model{ID: 1},
		},
	}

	// 無法建立文件系統
	{
		task.TaskProps.Parent = "test/not_exist"
		task.User = &model.User{
			Policy: model.Policy{
				Type: "unknown",
			},
		}
		mock.ExpectBegin()
		mock.ExpectExec("UPDATE(.+)").WillReturnResult(sqlmock.NewResult(1,
			1))
		mock.ExpectCommit()
		task.Do()
		asserts.NoError(mock.ExpectationsWereMet())
		asserts.NotEmpty(task.GetError().Msg)
	}

	// 上傳出錯
	{
		task.User = &model.User{
			Policy: model.Policy{
				Type: "mock",
			},
		}
		task.TaskProps.Src = []string{"test/not_exist"}
		task.TaskProps.Parent = "test/not_exist"
		// 更新進度
		mock.ExpectBegin()
		mock.ExpectExec("UPDATE(.+)").WillReturnResult(sqlmock.NewResult(1,
			1))
		mock.ExpectCommit()
		// 更新錯誤
		mock.ExpectBegin()
		mock.ExpectExec("UPDATE(.+)").WillReturnResult(sqlmock.NewResult(1,
			1))
		mock.ExpectCommit()
		task.Do()
		asserts.NoError(mock.ExpectationsWereMet())
		asserts.NotEmpty(task.GetError().Msg)
	}

	// 取代目錄前綴
	{
		task.User = &model.User{
			Policy: model.Policy{
				Type: "mock",
			},
		}
		task.TaskProps.Src = []string{"test/not_exist"}
		task.TaskProps.Parent = "test/not_exist"
		task.TaskProps.TrimPath = true
		// 更新進度
		mock.ExpectBegin()
		mock.ExpectExec("UPDATE(.+)").WillReturnResult(sqlmock.NewResult(1,
			1))
		mock.ExpectCommit()
		// 更新錯誤
		mock.ExpectBegin()
		mock.ExpectExec("UPDATE(.+)").WillReturnResult(sqlmock.NewResult(1,
			1))
		mock.ExpectCommit()
		task.Do()
		asserts.NoError(mock.ExpectationsWereMet())
		asserts.NotEmpty(task.GetError().Msg)
	}
}

func TestNewTransferTask(t *testing.T) {
	asserts := assert.New(t)

	// 成功
	{
		mock.ExpectQuery("SELECT(.+)").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
		mock.ExpectBegin()
		mock.ExpectExec("INSERT(.+)").WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()
		job, err := NewTransferTask(1, []string{}, "/", "/", false)
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
		job, err := NewTransferTask(1, []string{}, "/", "/", false)
		asserts.NoError(mock.ExpectationsWereMet())
		asserts.Nil(job)
		asserts.Error(err)
	}
}

func TestNewTransferTaskFromModel(t *testing.T) {
	asserts := assert.New(t)

	// 成功
	{
		mock.ExpectQuery("SELECT(.+)").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
		job, err := NewTransferTaskFromModel(&model.Task{Props: "{}"})
		asserts.NoError(mock.ExpectationsWereMet())
		asserts.NoError(err)
		asserts.NotNil(job)
	}

	// JSON解析失敗
	{
		mock.ExpectQuery("SELECT(.+)").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
		job, err := NewTransferTaskFromModel(&model.Task{Props: "?"})
		asserts.NoError(mock.ExpectationsWereMet())
		asserts.Error(err)
		asserts.Nil(job)
	}
}
