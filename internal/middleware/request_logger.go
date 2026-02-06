package middleware

import (
	"bytes"
	"io"
	"time"

	"cloudque/internal/model/entity"
	"cloudque/internal/service"
	"cloudque/pkg/jwt"

	"github.com/gin-gonic/gin"
)

// RequestLogger 请求日志中间件
func RequestLogger(logService service.LogService) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		// 读取请求体（注意：对于文件上传等大请求，不要读取全部内容）
		var body string
		if c.Request.Method == "POST" || c.Request.Method == "PUT" {
			// 如果是文件上传，不记录Body
			contentType := c.GetHeader("Content-Type")
			if contentType != "multipart/form-data" {
				bodyBytes, _ := io.ReadAll(c.Request.Body)
				// 重新赋值Body，供后续使用
				c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
				
				// 截取前1000个字符
				if len(bodyBytes) > 1000 {
					body = string(bodyBytes[:1000]) + "..."
				} else {
					body = string(bodyBytes)
				}
			} else {
				body = "[Multipart File Upload]"
			}
		}

		// 处理请求
		c.Next()

		// 结束时间
		end := time.Now()
		latency := end.Sub(start).Milliseconds()

		// 获取用户ID
		var userID uint
		if claims, exists := c.Get("claims"); exists {
			if customClaims, ok := claims.(*jwt.CustomClaims); ok {
				userID = customClaims.UserID
			}
		}

		// 记录日志
		log := &entity.RequestLog{
			UserID:     userID,
			Method:     c.Request.Method,
			Path:       path,
			Query:      query,
			Body:       body,
			IPAddress:  c.ClientIP(),
			UserAgent:  c.Request.UserAgent(),
			StatusCode: c.Writer.Status(),
			Latency:    latency,
		}

		logService.CreateRequestLog(log)
	}
}
