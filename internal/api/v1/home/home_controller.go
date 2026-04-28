package home

import (
	"cloudque/internal/middleware"
	"cloudque/internal/service"
	"cloudque/pkg/logger"
	"cloudque/pkg/response"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type HomeController struct {
	homeService             service.HomeService
	authService             service.AuthService
	userOperationLogService service.UserOperationLogService
}

func NewHomeController(homeService service.HomeService, authService service.AuthService, userOperationLogService service.UserOperationLogService) *HomeController {
	return &HomeController{
		homeService:             homeService,
		authService:             authService,
		userOperationLogService: userOperationLogService,
	}
}

func (ctrl *HomeController) Overview(c *gin.Context) {
	data, err := ctrl.homeService.GetOverview(c.Request.Context())
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.Success(c, data)
}

var homeUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func (ctrl *HomeController) WebSocketOverview(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		response.BadRequest(c, "未找到用户信息")
		return
	}

	conn, err := homeUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logger.Infof("首页概览websocket升级失败: %v", err)
		return
	}

	ctrl.homeService.HandleHomeMessage(conn, userID)
}
