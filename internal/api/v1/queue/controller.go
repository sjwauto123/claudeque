package queue

import (
	"cloudque/internal/middleware"
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
	queueService    service.QueueService
	jobRepo         repository.JobRepository
	operationLogSvc service.OperationLogService
}

// NewController 创建队列控制器
func NewController(queueService service.QueueService, jobRepo repository.JobRepository, operationLogSvc service.OperationLogService) *Controller {
	return &Controller{
		queueService:    queueService,
		jobRepo:         jobRepo,
		operationLogSvc: operationLogSvc,
	}
}

// GetQueue 获取排队队列
func (c *Controller) GetQueue(ctx *gin.Context) {
	var req request.JobListRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		response.BadRequest(ctx, err.Error())
		return
	}
	req.Status = 1
	startTime, err := utils.ParseTime(req.StartTime)
	if err != nil {
		response.BadRequest(ctx, "开始时间格式错误")
		return
	}
	endTime, err := utils.ParseTime(req.EndTime)
	if err != nil {
		response.BadRequest(ctx, "截至时间格式错误")
		return
	}

	list, total, page, pageSize, err := c.queueService.GetQueuePage(ctx.Request.Context(), req, startTime, endTime)
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

	username := middleware.GetUsername(c)

	job, err := ctrl.jobRepo.GetByID(req.JobID)
	if err != nil {
		if err := ctrl.operationLogSvc.Log(username, "拖拽任务", "", err.Error(), false); err != nil {
			logger.Warn("记录操作日志失败", zap.Error(err), zap.String("username", username))
		}
		response.InternalError(c, err.Error())
		return
	}

	filePath := ""
	if job != nil {
		filePath = job.FilePath
	}

	targetJobIDStr := utils.IntToString(req.TargetJobID)

	// 语义：把 JobID 插到 TargetJobID 前面
	if err := ctrl.queueService.MoveBefore(c.Request.Context(), req.JobID, req.TargetJobID); err != nil {
		if err := ctrl.operationLogSvc.Log(username, "拖拽任务", filePath, err.Error(), false); err != nil {
			logger.Warn("记录操作日志失败", zap.Error(err), zap.String("username", username))
		}
		response.InternalError(c, err.Error())
		return
	}

	description := "插入到任务" + targetJobIDStr + "前面"
	if err := ctrl.operationLogSvc.Log(username, "拖拽任务", filePath, description, true); err != nil {
		logger.Warn("记录操作日志失败", zap.Error(err), zap.String("username", username))
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

	username := middleware.GetUsername(c)

	job, err := ctrl.jobRepo.GetByID(jobID)
	if err != nil {
		if err := ctrl.operationLogSvc.Log(username, "删除排队任务", "", err.Error(), false); err != nil {
			logger.Warn("记录操作日志失败", zap.Error(err), zap.String("username", username))
		}
		response.InternalError(c, err.Error())
		return
	}

	filePath := ""
	if job != nil {
		filePath = job.FilePath
	}

	if err := ctrl.queueService.Remove(c.Request.Context(), jobID); err != nil {
		if err := ctrl.operationLogSvc.Log(username, "删除排队任务", filePath, err.Error(), false); err != nil {
			logger.Warn("记录操作日志失败", zap.Error(err), zap.String("username", username))
		}
		response.InternalError(c, err.Error())
		return
	}

	// 同步更新任务状态，避免数据库仍显示为排队中
	if job != nil && (job.Status == entity.JobStatusQueued || job.Status == entity.JobStatusWaitingGpu) {
		if err := ctrl.jobRepo.UpdateStatus(jobID, entity.JobStatusCancelled); err != nil {
			if err := ctrl.operationLogSvc.Log(username, "删除排队任务", filePath, err.Error(), false); err != nil {
				logger.Warn("记录操作日志失败", zap.Error(err), zap.String("username", username))
			}
			response.InternalError(c, err.Error())
			return
		}
	}

	if err := ctrl.operationLogSvc.Log(username, "删除排队任务", filePath, "成功", true); err != nil {
		logger.Warn("记录操作日志失败", zap.Error(err), zap.String("username", username))
	}
	response.Success(c, nil)
}
