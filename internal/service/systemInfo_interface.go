package service

import (
	"cloudque/internal/model/dto/response"

	"github.com/gorilla/websocket"
)

type SystemInfoService interface {
	// HandleSyMessage 处理 WebSocket 连接
	HandleSyMessage(conn *websocket.Conn, userID int)

	// Stop 停止服务
	Stop()

	// TerminateProcess 手动中断进程
	TerminateProcess(pid int) error

	// RetainProcess 保留进程（不被自动中断）
	RetainProcess(pid int) error

	// CancelRetain 取消保留
	CancelRetain(pid int) error

	// GetConfig 获取全局配置
	GetConfig() *response.ConfigResponse

	// UpdateConfig 更新全局配置
	UpdateConfig(enabled *bool, duration *int) error
}
