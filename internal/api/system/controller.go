// internal/api/system/system_info_controller.go （建议放在 api/system/）

package system

import (
	"cloudque/internal/service"
	"cloudque/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"net/http"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // 开发阶段可临时放开，生产务必限制
	},
}

type Controller struct {
	svc service.SystemInfoService
}

func NewController(svc service.SystemInfoService) *Controller {

	if svc == nil {
		panic("svc must not be nil")
	}
	return &Controller{
		svc: svc,
	}
}

func (ctrl *Controller) HandleWebSocket(c *gin.Context) {
	// 检查是否是管理员
	// userID := middleware.GetUserID(c) // 假设返回 string
	// role := middleware.GetUserRole(c)
	// if role != "admin" {
	// 	response.Forbidden(c, "仅管理员可访问")
	// 	return
	// }

	// 升级为WebSocket连接
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logger.Info("升级失败")
		// Gin 已接管 writer，不能写 JSON，直接 return
		return
	}

	// 处理WebSocket连接
	ctrl.svc.HandleSyMessage(conn)

}
