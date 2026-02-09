package operationLogs

import (
	"cloudque/internal/model/dto/request"
	"cloudque/internal/service"
	"cloudque/pkg/response"
	"github.com/gin-gonic/gin"
	"time"
)

type Controller struct {
	userOperationLogSer service.UserOperationLogService
	authService         service.AuthService
}

func NewController(userOperationLogCon service.UserOperationLogService, authService service.AuthService) *Controller {
	return &Controller{
		userOperationLogSer: userOperationLogCon,
		authService:         authService,
	}
}

func (ctrl *Controller) GetUserLogs(c *gin.Context) {
	var req request.UserLogsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}
	// 参数标准化
	page := req.Page
	if page < 1 {
		page = 1
	}
	size := req.Size
	if size < 1 {
		size = 10
	}
	if size > 100 {
		size = 100
	}

	var start, end time.Time
	if req.StartTime != "" {
		t, err := time.Parse("2006-01-02", req.StartTime)
		if err != nil {
			response.BadRequest(c, "开始时间格式错误，请使用YYYY-MM-DD格式")
			return
		}
		start = t
	}

	if req.EndTime != "" {
		t, err := time.Parse("2006-01-02", req.EndTime)
		if err != nil {
			response.BadRequest(c, "结束时间格式错误，请使用YYYY-MM-DD格式")
			return
		}
		// 结束时间扩展到当天 23:59:59
		t = t.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
		end = t
	}

	// 验证开始时间是否小于结束时间
	if !start.IsZero() && !end.IsZero() && start.After(end) {
		response.BadRequest(c, "开始时间不能晚于结束时间")
		return
	}

	data, err := ctrl.userOperationLogSer.GetUserLogs(&req, start, end)
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, data)

}
