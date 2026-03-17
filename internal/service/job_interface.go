package service

import (
	"cloudque/internal/model/dto/request"
	"cloudque/internal/model/dto/response"
	"cloudque/internal/model/entity"
	"context"
	"time"
)

type JobService interface {
	// GetJobList 获取任务列表
	GetJobList(req request.JobListRequest, startTime time.Time, endTime time.Time, userID int) ([]response.JobResponse, int64, int, int, error)
	// GetWaitJobList 获取正在排队的任务列表
	GetWaitJobList(req request.JobListRequest, startTime time.Time, endTime time.Time, userID int) ([]response.JobResponse, int64, int, int, error)
	// SubmitJob 提交任务
	SubmitJob(ctx context.Context, req request.SubmitJobRequest, userID int) (*entity.Job, error)
	// CancelJob 取消任务排队
	CancelJob(ctx context.Context, jobID int, userID int) error
	// EnrichJobList 添加队列位置信息
	EnrichJobList(ctx context.Context, jobs []response.JobResponse) error
	// GetJobByID 根据ID获取任务信息
	GetJobByID(jobID int) (*entity.Job, error)
	// GetStats 获取任务统计
	GetStats() (*response.JobStatsResponse, error)
	// ListCondaEnvs 获取 Conda 环境列表
	ListCondaEnvs(ctx context.Context, userID int) ([]string, error)
	// SetScheduler 设置调度器
	SetScheduler(scheduler Scheduler)
}
