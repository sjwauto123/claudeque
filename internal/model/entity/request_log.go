package entity

import "time"

// RequestLog 用户请求日志实体
type RequestLog struct {
	ID         uint64    `gorm:"primaryKey;autoIncrement;comment:日志ID" json:"id"`
	UserID     uint      `gorm:"type:int unsigned;default:0;comment:用户ID;index:idx_user_request" json:"user_id"`
	Method     string    `gorm:"type:varchar(10);not null;comment:请求方法" json:"method"`
	Path       string    `gorm:"type:varchar(255);not null;comment:请求路径;index:idx_path" json:"path"`
	Query      string    `gorm:"type:text;comment:请求参数" json:"query"`
	Body       string    `gorm:"type:text;comment:请求体" json:"body"`
	IPAddress  string    `gorm:"type:varchar(45);not null;comment:请求IP" json:"ip_address"`
	UserAgent  string    `gorm:"type:varchar(255);default:'';comment:用户代理" json:"user_agent"`
	StatusCode int       `gorm:"type:int;not null;comment:响应状态码" json:"status_code"`
	Latency    int64     `gorm:"type:bigint;default:0;comment:耗时(ms)" json:"latency"`
	CreatedAt  time.Time `gorm:"type:timestamp;default:CURRENT_TIMESTAMP;comment:请求时间;index:idx_request_time;index:idx_user_request" json:"created_at"`
}

// TableName 指定表名
func (RequestLog) TableName() string {
	return "request_logs"
}
