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
	authService             service.AuthService
	userOperationLogService service.UserOperationLogService
}

func NewController(svc service.SystemInfoService, authSvc service.AuthService, userOperationLogService service.UserOperationLogService) *Controller {
	return &Controller{
		syInfoSvc:               svc,
		authService:             authSvc,
		userOperationLogService: userOperationLogService,
	}
}

// NewWebsocketUpgrader 创建WebSocket升级器
func NewWebsocketUpgrader() *websocket.Upgrader {
	return &websocket.Upgrader{
		// 允许的来源列表
		CheckOrigin: func(r *http.Request) bool {
			//origin := r.Header.Get("Origin")
			//allowedOrigins := []string{
			//	"https://yourdomain.com",
			//	"http://localhost:3000", // 开发环境允许本地连接
			//	"http://127.0.0.1:3000",
			//}
			//
			//// 检查来源是否在允许列表中
			//for _, allowed := range allowedOrigins {
			//	if origin == allowed {
			//		return true
			//	}
			//}
			//
			//// 生产环境建议严格检查，开发环境可以暂时返回true
			//// return false
			return true
		},

		// 配置WebSocket参数
		ReadBufferSize:  1024, // 读取缓冲区大小
		WriteBufferSize: 1024, // 写入缓冲区大小
	}
}

// 全局WebSocket升级器实例
var upgrader = NewWebsocketUpgrader()

func (ctrl *Controller) HandleWebSocket(c *gin.Context) {

	value, exists := c.Get(middleware.ContextUserID)
	if !exists {
		response.BadRequest(c, "未找到用户信息")
		return
	}

	userID := value.(int)

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
