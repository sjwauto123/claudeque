package job

import (
	"cloudque/internal/middleware"
	"cloudque/internal/model/dto/request"
	dtoResponse "cloudque/internal/model/dto/response"
	"cloudque/internal/service"
	"cloudque/pkg/response"
	"cloudque/pkg/utils"
	"time"

	"github.com/gin-gonic/gin"
)

// Controller 任务控制器
type Controller struct {
	jobService              service.JobService
	authService             service.AuthService
	gpuService              service.GpuService
	userOperationLogService service.UserOperationLogService
}

// NewController 创建任务控制器
func NewController(jobService service.JobService, authService service.AuthService, gpuService service.GpuService, userOperationLogService service.UserOperationLogService) *Controller {
	return &Controller{
		jobService:              jobService,
		authService:             authService,
		gpuService:              gpuService,
		userOperationLogService: userOperationLogService,
	}
}

// GetJobsList 获取任务列表
func (ctrl *Controller) GetJobsList(c *gin.Context) {
	ctrl.handleJobList(c, ctrl.jobService.GetJobList)
}

// GetWaitJobsList 获取排队中的任务列表
func (ctrl *Controller) GetWaitJobsList(c *gin.Context) {
	ctrl.handleJobList(c, ctrl.jobService.GetWaitJobList)
}

// handleJobList 内部公共方法：处理任务列表请求
func (ctrl *Controller) handleJobList(c *gin.Context, fetchFunc func(request.JobListRequest, time.Time, time.Time, int) ([]dtoResponse.JobResponse, int64, int, int, error)) {
	var j request.JobListRequest
	if err := c.ShouldBindQuery(&j); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	userID := middleware.GetUserID(c)
	startTime, err := utils.ParseStartDate(j.StartTime)
	if err != nil {
		response.BadRequest(c, "开始时间格式错误")
		return
	}

	endTime, err := utils.ParseEndDate(j.EndTime)
	if err != nil {
		response.BadRequest(c, "截至时间格式错误")
		return
	}

	list, total, page, pageSize, err := fetchFunc(j, startTime, endTime, userID)
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
		response.BizError(c, err)
		return
	}

	response.Success(c, job)
}

// CancelJob 取消排队中的任务
func (ctrl *Controller) CancelJob(c *gin.Context) {
	jobIDStr := c.Param("id")
	if jobIDStr == "" {
		response.BadRequest(c, "缺少任务ID")
		return
	}

	var jobID int
	if _, err := utils.StringToInt(jobIDStr, &jobID); err != nil {
		response.BadRequest(c, "任务ID格式错误")
		return
	}

	userID := middleware.GetUserID(c)
	if err := ctrl.jobService.CancelJob(c.Request.Context(), jobID, userID); err != nil {
		response.Error(c, 4001, err.Error())
		return
	}

	response.Success(c, nil)
}

// GetStats 获取任务统计
func (ctrl *Controller) GetStats(c *gin.Context) {
	stats, err := ctrl.jobService.GetStats()
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, stats)
}

func (ctrl *Controller) GetGpus(c *gin.Context) {
	gpus, err := ctrl.gpuService.GetGpus(c)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, gpus)
}

// GetCondaEnvs 取提交者用户下的Conda环境
func (ctrl *Controller) GetCondaEnvs(c *gin.Context) {
	username := middleware.GetUsername(c)
	envs, err := ctrl.jobService.ListCondaEnvs(c, username)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, envs)
}
