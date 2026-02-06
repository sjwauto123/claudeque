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
	GetJobList(req request.JobListRequest, startTime time.Time, endTime time.Time, userID uint) ([]response.JobResponse, int64, int, int, error)
	// SubmitJob 提交任务
	SubmitJob(ctx context.Context, req request.SubmitJobRequest, userID uint) (*entity.Job, error)
	// CancelJob 取消任务排队
	CancelJob(ctx context.Context, jobID uint, userID uint) error
	// EnrichJobList 添加队列位置信息
	EnrichJobList(ctx context.Context, jobs []response.JobResponse) error
	// GetJobLog 查看任务日志
	GetJobLog(ctx context.Context, jobID uint, userID uint) (string, error)
	// GetJobByID 根据ID获取任务信息
	GetJobByID(ctx context.Context, jobID uint) (*entity.Job, error)
	// GetStats 获取任务统计
	GetStats() (*response.JobStatsResponse, error)
}
