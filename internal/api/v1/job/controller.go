package job

import (
	"cloudque/internal/middleware"
	"cloudque/internal/model/dto/request"
	"cloudque/internal/service"
	"cloudque/pkg/response"
	"cloudque/pkg/utils"
	"github.com/gin-gonic/gin"
)

// Controller 任务控制器
type Controller struct {
	jobService service.JobService
}

// NewController 创建任务控制器
func NewController(jobService service.JobService) *Controller {
	return &Controller{
		jobService: jobService,
	}
}

// GetJobsList 获取任务列表
func (ctrl *Controller) GetJobsList(c *gin.Context) {
	var j request.JobListRequest
	if err := c.ShouldBindQuery(&j); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	userID := middleware.GetUserID(c)
	startTime, err := utils.ParseTime(j.StartTime)
	if err != nil {
		response.BadRequest(c, "开始时间格式错误")
		return
	}

	endTime, err := utils.ParseTime(j.EndTime)
	if err != nil {
		response.BadRequest(c, "截至时间格式错误")
		return
	}

	list, total, page, pageSize, err := ctrl.jobService.GetJobList(j, startTime, endTime, userID)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	if err := ctrl.jobService.EnrichJobList(c.Request.Context(), list); err != nil {
		response.Error(c, 500, err.Error())
		return
	}

	resp := response.NewPageResponse(
		list,
		total,
		page,
		pageSize,
	)

	response.Success(c, resp)
}

// GetWaitJobsList 获取排队中的任务列表
func (ctrl *Controller) GetWaitJobsList(c *gin.Context) {
	var j request.JobListRequest
	if err := c.ShouldBindQuery(&j); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	j.Status = 1
	userID := middleware.GetUserID(c)
	startTime, err := utils.ParseTime(j.StartTime)
	if err != nil {
		response.BadRequest(c, "开始时间格式错误")
		return
	}

	endTime, err := utils.ParseTime(j.EndTime)
	if err != nil {
		response.BadRequest(c, "截至时间格式错误")
		return
	}

	list, total, page, pageSize, err := ctrl.jobService.GetJobList(j, startTime, endTime, userID)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	if err := ctrl.jobService.EnrichJobList(c.Request.Context(), list); err != nil {
		response.Error(c, 500, err.Error())
		return
	}

	resp := response.NewPageResponse(
		list,
		total,
		page,
		pageSize,
	)

	response.Success(c, resp)
}

// SubmitJob 提交任务
func (ctrl *Controller) SubmitJob(c *gin.Context) {
	var req request.SubmitJobRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	userID := middleware.GetUserID(c)

	job, err := ctrl.jobService.SubmitJob(c.Request.Context(), req, userID)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.Success(c, job)
}

// CancelJob 取消排队中的任务
func (ctrl *Controller) CancelJob(c *gin.Context) {
	// 获取job_id参数
	jobIDStr := c.Param("jobId")
	if jobIDStr == "" {
		response.BadRequest(c, "缺少任务ID")
		return
	}

	var jobID uint
	if _, err := utils.StringToUint(jobIDStr, &jobID); err != nil {
		response.BadRequest(c, "任务ID格式错误")
		return
	}

	userID := middleware.GetUserID(c)
	if err := ctrl.jobService.CancelJob(c.Request.Context(), jobID, userID); err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.Success(c, nil)
}

// GetJobLog 查看任务日志
func (ctrl *Controller) GetJobLog(c *gin.Context) {
	jobIDStr := c.Param("jobId")
	if jobIDStr == "" {
		response.BadRequest(c, "缺少任务ID")
		return
	}

	var jobID uint
	if _, err := utils.StringToUint(jobIDStr, &jobID); err != nil {
		response.BadRequest(c, "任务ID格式错误")
		return
	}

	userID := middleware.GetUserID(c)
	log, err := ctrl.jobService.GetJobLog(c.Request.Context(), jobID, userID)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, gin.H{"log": log})
}
