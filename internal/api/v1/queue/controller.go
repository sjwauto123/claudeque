package queue

import (
	"cloudque/internal/model/dto/request"
	"cloudque/internal/model/entity"
	"cloudque/internal/repository"
	"cloudque/internal/service"
	"cloudque/pkg/logger"
	"cloudque/pkg/response"
	"cloudque/pkg/utils"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Controller 队列控制器
type Controller struct {
	queueService            service.QueueService
	userOperationLogService service.UserOperationLogService
	jobRepo                 repository.JobRepository
	authService             service.AuthService
}

// NewController 创建队列控制器
func NewController(queueService service.QueueService, userOperationLogService service.UserOperationLogService, jobRepo repository.JobRepository, authService service.AuthService) *Controller {
	return &Controller{
		queueService:            queueService,
		userOperationLogService: userOperationLogService,
		jobRepo:                 jobRepo,
		authService:             authService,
	}
}

// GetQueue 获取排队队列
func (ctrl *Controller) GetQueue(ctx *gin.Context) {
	var req request.QueueListRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		response.BadRequest(ctx, err.Error())
		return
	}
	startTime, err := utils.ParseStartDate(req.StartTime)
	if err != nil {
		response.BadRequest(ctx, "开始时间格式错误")
		return
	}
	endTime, err := utils.ParseEndDate(req.EndTime)
	if err != nil {
		response.BadRequest(ctx, "截至时间格式错误")
		return
	}

	list, total, page, pageSize, err := ctrl.queueService.GetQueuePage(ctx.Request.Context(), req, startTime, endTime)
	if err != nil {
		response.InternalError(ctx, err.Error())
		return
	}

	resp := response.NewPageResponse(list, total, page, pageSize)
	response.Success(ctx, resp)
}

// ReorderQueue 调整队列顺序
func (ctrl *Controller) ReorderQueue(c *gin.Context) {
	var req request.ReorderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// 获取 JobID 任务信息
	job, err := ctrl.jobRepo.GetByID(req.JobID)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	// 检查 JobID 任务状态，如果是正在执行中，则不允许重排
	if job.Status == entity.JobStatusRunning {
		response.BadRequest(c, "正在执行中的任务不允许重排")
		return
	}

	// 获取 TargetJobID 任务信息
	targetJob, err := ctrl.jobRepo.GetByID(req.TargetJobID)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	// 检查 TargetJobID 任务状态，如果是正在执行中，则不允许重排到其前面
	if targetJob.Status == entity.JobStatusRunning {
		response.BadRequest(c, "不允许将任务重排到正在执行中的任务前面")
		return
	}
	// 把 JobID 插到 TargetJobID 前面
	if err := ctrl.queueService.MoveBefore(c.Request.Context(), req.JobID, req.TargetJobID); err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.Success(c, nil)
}

// RemoveJob 从队列中移除任务
func (ctrl *Controller) RemoveJob(c *gin.Context) {
	jobIDStr := c.Param("jobId")
	if jobIDStr == "" {
		response.BadRequest(c, "缺少任务ID")
		return
	}

	var jobID int
	if _, err := utils.StringToInt(jobIDStr, &jobID); err != nil {
		response.BadRequest(c, "任务ID格式错误")
		return
	}

	job, err := ctrl.jobRepo.GetByID(jobID)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	if err := ctrl.queueService.Remove(c.Request.Context(), jobID); err != nil {
		response.InternalError(c, err.Error())
		return
	}

	// 同步更新任务状态，避免数据库仍显示为排队中
	if job != nil && (job.Status == entity.JobStatusQueued || job.Status == entity.JobStatusWaitingGpu) {
		if err := ctrl.jobRepo.UpdateStatus(jobID, entity.JobStatusCancelled); err != nil {
			response.InternalError(c, err.Error())
			return
		}
		logger.Info("任务状态更新为：已取消 (移除队列)", zap.Int("job_id", jobID))
	}

	response.Success(c, nil)
}
