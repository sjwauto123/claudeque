package system

import (
	"cloudque/internal/middleware"
	"cloudque/internal/model/dto/request"
	"cloudque/internal/service"
	"cloudque/pkg/logger"
	"cloudque/pkg/response"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type Controller struct {
	syInfoSvc   service.SystemInfoService
	authService service.AuthService
}

func NewController(svc service.SystemInfoService, authSvc service.AuthService) *Controller {
	return &Controller{
		syInfoSvc:   svc,
		authService: authSvc,
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

	//类型断言
	userID, ok := value.(int)
	if !ok {
		response.BadRequest(c, "用户信息类型错误")
		return
	}

	// 升级为WebSocket连接
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)

	defer conn.Close()
	if err != nil {
		logger.Info("Failed to upgrade to WebSocket:")
		return
	}
	// 处理WebSocket连接
	ctrl.syInfoSvc.HandleSyMessage(conn, userID)

}

// TerminateProcess 手动中断进程
func (ctrl *Controller) TerminateProcess(c *gin.Context) {
	pid, err := strconv.Atoi(c.Param("pid"))
	if err != nil {
		response.BadRequest(c, "无效的 PID")
		return
	}

	err = ctrl.syInfoSvc.TerminateProcess(pid)
	if err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, "进程已终止")
}

// RetainProcess 保留进程
func (ctrl *Controller) RetainProcess(c *gin.Context) {
	pid, err := strconv.Atoi(c.Param("pid"))
	if err != nil {
		response.BadRequest(c, "无效的 PID")
		return
	}

	err = ctrl.syInfoSvc.RetainProcess(pid)
	if err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, "已保留该进程")
}

// CancelRetain 取消保留
func (ctrl *Controller) CancelRetain(c *gin.Context) {
	pid, err := strconv.Atoi(c.Param("pid"))
	if err != nil {
		response.BadRequest(c, "无效的 PID")
		return
	}

	err = ctrl.syInfoSvc.CancelRetain(pid)
	if err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, "已取消保留")
}

// GetConfig 获取全局配置
func (ctrl *Controller) GetConfig(c *gin.Context) {
	config := ctrl.syInfoSvc.GetConfig()
	response.Success(c, config)
}

// UpdateConfig 更新全局配置
func (ctrl *Controller) UpdateConfig(c *gin.Context) {
	var req request.UpdateConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}

	if req.MaxDurationMinutes != nil && *req.MaxDurationMinutes < 1 {
		response.BadRequest(c, "最大时长必须大于0")
		return
	}

	err := ctrl.syInfoSvc.UpdateConfig(req.AutoTerminateEnabled, req.MaxDurationMinutes)
	if err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, "配置已更新")
}
