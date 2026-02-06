package userLog

import (
	"cloudque/internal/model/dto/request"
	"cloudque/internal/service"
	"cloudque/pkg/response"
	"github.com/gin-gonic/gin"
)

type Controller struct {
	userLogSer service.UserLogService
}

func NewController(userLogCon service.UserLogService) *Controller {
	return &Controller{
		userLogSer: userLogCon,
	}
}

func (ctrl *Controller) GetUserLogs(c *gin.Context) {
	var req request.UserLogsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	data, err := ctrl.userLogSer.GetUserLogs(&req)
	if err != nil {
		response.BizError(c, err)
		return

	}

	response.Success(c, data)

}
