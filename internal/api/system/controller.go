// internal/api/system/system_info_controller.go （建议放在 api/system/）

package system

import (
	"cloudque/internal/middleware"
	"cloudque/internal/service"
	"cloudque/pkg/logger"
	"cloudque/pkg/response"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // 开发阶段可临时放开，生产务必限制
	},
}

type Controller struct {
	syInfoSvc service.SystemInfoService
	authSvc   service.AuthService
}

func NewController(svc service.SystemInfoService, authSvc service.AuthService) *Controller {
	return &Controller{
		syInfoSvc: svc,
		authSvc:   authSvc,
	}
}

func (ctrl *Controller) HandleWebSocket(c *gin.Context) {

	value, exists := c.Get(middleware.ContextUserID)
	if !exists {
		response.BadRequest(c, "未找到用户信息")
		return
	}

	userID := value.(uint)

	// 升级为WebSocket连接
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logger.Info("升级失败")
		// Gin 已接管 writer，不能写 JSON，直接 return
		return
	}
	// 处理WebSocket连接
	ctrl.syInfoSvc.HandleSyMessage(conn, userID)
}
