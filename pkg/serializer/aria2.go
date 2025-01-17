package serializer

import (
	"path"
	"time"

	model "github.com/cloudreve/Cloudreve/v3/models"
	"github.com/cloudreve/Cloudreve/v3/pkg/aria2/rpc"
)

// DownloadListResponse 下載列表響應條目
type DownloadListResponse struct {
	UpdateTime     time.Time      `json:"update"`
	UpdateInterval int            `json:"interval"`
	Name           string         `json:"name"`
	Status         int            `json:"status"`
	Dst            string         `json:"dst"`
	Total          uint64         `json:"total"`
	Downloaded     uint64         `json:"downloaded"`
	Speed          int            `json:"speed"`
	Info           rpc.StatusInfo `json:"info"`
}

// FinishedListResponse 已完成任務條目
type FinishedListResponse struct {
	Name       string         `json:"name"`
	GID        string         `json:"gid"`
	Status     int            `json:"status"`
	Dst        string         `json:"dst"`
	Error      string         `json:"error"`
	Total      uint64         `json:"total"`
	Files      []rpc.FileInfo `json:"files"`
	TaskStatus int            `json:"task_status"`
	TaskError  string         `json:"task_error"`
	CreateTime time.Time      `json:"create"`
	UpdateTime time.Time      `json:"update"`
}

// BuildFinishedListResponse 構建已完成任務條目
func BuildFinishedListResponse(tasks []model.Download) Response {
	resp := make([]FinishedListResponse, 0, len(tasks))

	for i := 0; i < len(tasks); i++ {
		fileName := tasks[i].StatusInfo.BitTorrent.Info.Name
		if len(tasks[i].StatusInfo.Files) == 1 {
			fileName = path.Base(tasks[i].StatusInfo.Files[0].Path)
		}

		// 過濾敏感訊息
		for i2 := 0; i2 < len(tasks[i].StatusInfo.Files); i2++ {
			tasks[i].StatusInfo.Files[i2].Path = path.Base(tasks[i].StatusInfo.Files[i2].Path)
		}

		download := FinishedListResponse{
			Name:       fileName,
			GID:        tasks[i].GID,
			Status:     tasks[i].Status,
			Error:      tasks[i].Error,
			Dst:        tasks[i].Dst,
			Total:      tasks[i].TotalSize,
			Files:      tasks[i].StatusInfo.Files,
			TaskStatus: -1,
			UpdateTime: tasks[i].UpdatedAt,
			CreateTime: tasks[i].CreatedAt,
		}

		if tasks[i].Task != nil {
			download.TaskError = tasks[i].Task.Error
			download.TaskStatus = tasks[i].Task.Status
		}

		resp = append(resp, download)
	}

	return Response{Data: resp}
}

// BuildDownloadingResponse 構建正在下載的列表響應
func BuildDownloadingResponse(tasks []model.Download) Response {
	resp := make([]DownloadListResponse, 0, len(tasks))
	interval := model.GetIntSetting("aria2_interval", 10)

	for i := 0; i < len(tasks); i++ {
		fileName := ""
		if len(tasks[i].StatusInfo.Files) > 0 {
			fileName = path.Base(tasks[i].StatusInfo.Files[0].Path)
		}

		// 過濾敏感訊息
		tasks[i].StatusInfo.Dir = ""
		for i2 := 0; i2 < len(tasks[i].StatusInfo.Files); i2++ {
			tasks[i].StatusInfo.Files[i2].Path = path.Base(tasks[i].StatusInfo.Files[i2].Path)
		}

		resp = append(resp, DownloadListResponse{
			UpdateTime:     tasks[i].UpdatedAt,
			UpdateInterval: interval,
			Name:           fileName,
			Status:         tasks[i].Status,
			Dst:            tasks[i].Dst,
			Total:          tasks[i].TotalSize,
			Downloaded:     tasks[i].DownloadedSize,
			Speed:          tasks[i].Speed,
			Info:           tasks[i].StatusInfo,
		})
	}

	return Response{Data: resp}
}
