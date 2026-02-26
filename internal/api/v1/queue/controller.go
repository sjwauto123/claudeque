package queue

import (
	"cloudque/internal/model/dto/request"
	"cloudque/internal/model/entity"
	"cloudque/internal/repository"
	"cloudque/internal/service"
	"cloudque/pkg/response"
	"cloudque/pkg/utils"
	"github.com/gin-gonic/gin"
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
	startTime, err := utils.ParseDateToStart(req.StartTime)
	if err != nil {
		response.BadRequest(ctx, "开始时间格式错误")
		return
	}
	endTime, err := utils.ParseDateToEnd(req.EndTime)
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
	if _, err := ctrl.jobRepo.GetByID(req.JobID); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	if _, err := ctrl.jobRepo.GetByID(req.TargetJobID); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	// 语义：把 JobID 插到 TargetJobID 前面
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
	}

	response.Success(c, nil)
}
