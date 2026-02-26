package config

import "time"

// Config 应用配置结构体
type Config struct {
	App      AppConfig      `mapstructure:"app"`
	Database DatabaseConfig `mapstructure:"database"`
	JWT      JWTConfig      `mapstructure:"jwt"`
	Log      LogConfig      `mapstructure:"log"`
	CORS     CORSConfig     `mapstructure:"cors"`
	Email    EmailConfig    `mapstructure:"email"`
	Server   ServerConfig   `mapstructure:"server"`
}

// AppConfig 应用配置
type AppConfig struct {
	Name      string `mapstructure:"name"`
	Version   string `mapstructure:"version"`
	Mode      string `mapstructure:"mode"` // debug, release, test
	Port      int    `mapstructure:"port"`
	AESSecret string `mapstructure:"aes_secret"`
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	MySQL MySQLConfig `mapstructure:"mysql"`
	Redis RedisConfig `mapstructure:"redis"`
}

// MySQLConfig MySQL 配置
type MySQLConfig struct {
	Host         string `mapstructure:"host"`
	Port         int    `mapstructure:"port"`
	Username     string `mapstructure:"username"`
	Password     string `mapstructure:"password"`
	Database     string `mapstructure:"database"`
	MaxIdleConns int    `mapstructure:"max_idle_conns"`
	MaxOpenConns int    `mapstructure:"max_open_conns"`
}

// RedisConfig Redis 配置
type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
	PoolSize int    `mapstructure:"pool_size"`
}

// JWTConfig JWT 配置
type JWTConfig struct {
	Secret      string        `mapstructure:"secret"`
	ExpireHours time.Duration `mapstructure:"expire_hours"`
}

// EmailConfig 邮件配置
type EmailConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
}

// LogConfig 日志配置
type LogConfig struct {
	Level      string `mapstructure:"level"`       // debug, info, warn, error
	Filename   string `mapstructure:"filename"`    // 日志文件路径
	MaxSize    int    `mapstructure:"max_size"`    // 单个日志文件最大大小(MB)
	MaxBackups int    `mapstructure:"max_backups"` // 保留的旧日志文件数量
	MaxAge     int    `mapstructure:"max_age"`     // 保留旧日志文件的最大天数
	Compress   bool   `mapstructure:"compress"`    // 是否压缩
}

// CORSConfig CORS 配置
type CORSConfig struct {
	Enabled          bool     `mapstructure:"enabled"`
	AllowOrigins     []string `mapstructure:"allow_origins"`
	AllowMethods     []string `mapstructure:"allow_methods"`
	AllowHeaders     []string `mapstructure:"allow_headers"`
	ExposeHeaders    []string `mapstructure:"expose_headers"`
	AllowCredentials bool     `mapstructure:"allow_credentials"`
	MaxAge           int      `mapstructure:"max_age"`
}

// ServerConfig 训练服务器配置
type ServerConfig struct {
	Host           string        `mapstructure:"host"`            // 服务器地址，如 "192.168.1.100:22"
	RootUsername   string        `mapstructure:"root_username"`   // 管理员SSH用户名（默认root）
	RootPassword   string        `mapstructure:"root_password"`   // 管理员SSH密码
	BasePath       string        `mapstructure:"base_path"`       // 服务器上的基础文件路径
	Timeout        time.Duration `mapstructure:"timeout"`         // 连接超时
	SessionTimeout time.Duration `mapstructure:"session_timeout"` // 会话超时时间，0表示永不超时
	Enabled        bool          `mapstructure:"enabled"`         // 是否启用训练服务器
}
