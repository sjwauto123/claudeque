package request

// UpdateConfigRequest 更新配置请求
type UpdateConfigRequest struct {
	AutoTerminateEnabled *bool `json:"auto_terminate_enabled"`
	MaxDurationMinutes   *int  `json:"max_duration_minutes"`
}
