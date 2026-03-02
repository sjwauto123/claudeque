package repository

import (
	"cloudque/internal/model/dto/request"
	"cloudque/internal/model/dto/response"
	"cloudque/internal/model/entity"
	"time"
)

// JobRepository 任务仓储接口
type JobRepository interface {
	// GetJobList 获取任务列表
	GetJobList(req request.JobListRequest, startTime time.Time, endTime time.Time, userID int) ([]response.JobResponse, int64, int, int, error)
	// GetWaitJobList 获取正在排队的任务列表
	GetWaitJobList(req request.JobListRequest, startTime time.Time, endTime time.Time, userID int) ([]response.JobResponse, int64, int, int, error)
	// Create 新建任务
	Create(job *entity.Job) error
	// GetByID 根据ID获得任务信息
	GetByID(id int) (*entity.Job, error)
	// UpdateStatus 更新任务状态
	UpdateStatus(id int, status int) error
	// GetQueueJobsByIDs 通过任务id获取任务信息
	GetQueueJobsByIDs(jobIDs []int) (map[int]response.QueueJobDBRow, error)
	// GetQueueJobListFiltered 按队列顺序、条件筛选、分页获取排队任务
	GetQueueJobListFiltered(orderedJobIDs []int, req request.QueueListRequest, startTime, endTime time.Time) ([]response.QueueJobDBRow, int, error)
	// GetRunningJobs 获取执行中的任务
	GetRunningJobs(req request.QueueListRequest, startTime, endTime time.Time) ([]response.QueueJobDBRow, int, error)
	// GetStats 获取任务统计
	GetStats() (*response.JobStatsResponse, error)
}
