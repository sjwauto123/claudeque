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

type Controller struct {
	syInfoSvc               service.SystemInfoService
	authSvc                 service.AuthService
	userOperationLogService service.UserOperationLogService
}

func NewController(svc service.SystemInfoService, authSvc service.AuthService, userOperationLogService service.UserOperationLogService) *Controller {
	return &Controller{
		syInfoSvc:               svc,
		authSvc:                 authSvc,
		userOperationLogService: userOperationLogService,
	}
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin") // 获取请求来源
		allowedOrigins := []string{
			"https://yourdomain.com",
			//"http://localhost:3000",
		}
		for _, allowed := range allowedOrigins {
			if origin == allowed {
				return true // 白名单中的来源允许连接
			}
		}
		return false // 其他来源拒绝连接
	},
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
		logger.Info("Failed to upgrade to WebSocket:")
		// Gin 已接管 writer，不能写 JSON，直接 return
		return
	}
	// 处理WebSocket连接
	ctrl.syInfoSvc.HandleSyMessage(conn, userID)
}
