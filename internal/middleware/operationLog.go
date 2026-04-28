package middleware

import (
	"bytes"
	"cloudque/internal/model/entity"
	"cloudque/internal/service"
	"cloudque/pkg/logger"
	"encoding/json"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	LogQueueSize   = 2000
	MaxBodyLogSize = 4 * 1024 // 仅记录前 4KB
)

// GlobalLogManager 全局日志管理器实例
var GlobalLogManager *LogManager

// LogManager 管理日志队列和消费者协程
type LogManager struct {
	logChan chan *entity.UserOperationLog
	wg      sync.WaitGroup
}

// NewLogManager 创建日志管理器
func NewLogManager(userLogService service.UserOperationLogService) *LogManager {
	lm := &LogManager{
		logChan: make(chan *entity.UserOperationLog, LogQueueSize),
	}

	lm.wg.Add(1)
	go func() {
		defer lm.wg.Done()
		for logEntry := range lm.logChan {
			if err := userLogService.CreateLog(logEntry); err != nil {
				logger.Errorf("用户操作日志保存失败,error:%v,path:%v", err, logEntry.Path)
			}
		}
	}()

	// 设置为全局实例
	GlobalLogManager = lm

	return lm
}

// Close 优雅关闭日志系统，等待剩余日志写入
func (lm *LogManager) Close() {
	close(lm.logChan)
	lm.wg.Wait()
	logger.Info("操作日志系统已关闭")
}

// UserOperationLogs 返回中间件函数
func (lm *LogManager) UserOperationLogs() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. 记录开始时间
		startTime := time.Now()

		// 2. 包装响应写入器
		blw := &bodyLogWriter{
			ResponseWriter: c.Writer,
			body:           bytes.NewBuffer(make([]byte, 0, 512)),
			maxBodySize:    MaxBodyLogSize,
		}
		c.Writer = blw

		// 3. 处理请求
		c.Next()

		// 4. 获取用户信息
		username := GetUsername(c)
		if username == "" {
			return
		}

		// 5. 获取操作类型
		actionType := ""
		if opType, exists := c.Get("operationType"); exists {
			if typeStr, ok := opType.(string); ok && typeStr != "" {
				actionType = typeStr
			}
		} else {
			return
		}

		// 6. 构建请求数据快照 (安全，不破坏 Body)
		requestData := buildRequestSnapshot(c)

		// 7. 获取状态和错误
		status, errorMessage := getBusinessStatus(blw.Body())
		if status == 0 {
			status = blw.Status()
		}
		if errorMessage == "" && len(c.Errors) > 0 {
			errorMessage = c.Errors.String()
		}

		// 8. 构建日志对象
		logEntry := &entity.UserOperationLog{
			Username:     username,
			Method:       c.Request.Method,
			Path:         c.Request.URL.Path,
			ActionType:   actionType,
			RequestData:  requestData,
			Status:       status,
			ErrorMessage: errorMessage,
			CreatedAt:    startTime,
		}

		// 9. 发送到队列
		select {
		case lm.logChan <- logEntry:
		default:
			logger.Infof("操作日志队列已满，丢弃当前日志,path:%v", logEntry.Path)
		}
	}
}

// bodyLogWriter 包装 gin.ResponseWriter 以捕获响应状态码和响应体
type bodyLogWriter struct {
	gin.ResponseWriter
	body        *bytes.Buffer
	statusCode  int
	maxBodySize int
}

func (w *bodyLogWriter) Write(b []byte) (int, error) {
	if w.body.Len() < w.maxBodySize {
		remaining := w.maxBodySize - w.body.Len()
		if len(b) > remaining {
			w.body.Write(b[:remaining])
			w.body.WriteString("[truncated]")
		} else {
			w.body.Write(b)
		}
	}
	return w.ResponseWriter.Write(b)
}

func (w *bodyLogWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *bodyLogWriter) Status() int {
	if w.statusCode == 0 {
		return w.ResponseWriter.Status()
	}
	return w.statusCode
}

func (w *bodyLogWriter) Body() []byte {
	return w.body.Bytes()
}

// buildRequestSnapshot 安全构建请求参数字符串
func buildRequestSnapshot(c *gin.Context) string {
	var parts []string

	// 1. Query Params
	for k, v := range c.Request.URL.Query() {
		for _, val := range v {
			parts = append(parts, k+"="+val)
		}
	}

	// 2. Path Params
	for _, param := range c.Params {
		parts = append(parts, param.Key+"="+param.Value)
	}

	// 3. Form / Body Data
	contentType := c.Request.Header.Get("Content-Type")
	var bodyContent string

	// 表单或文件上传 (Gin 已解析到 PostForm/MultipartForm)
	if strings.Contains(contentType, "application/x-www-form-urlencoded") ||
		strings.Contains(contentType, "multipart/form-data") {

		// 普通表单字段
		for k, v := range c.Request.PostForm {
			for _, val := range v {
				parts = append(parts, k+"="+val)
			}
		}

		// 文件字段 (只记录文件名)
		if c.Request.MultipartForm != nil {
			fileInfo := make(map[string]string)
			for fieldName, files := range c.Request.MultipartForm.File {
				if len(files) > 0 {
					fileInfo[fieldName] = files[0].Filename
				}
			}
			if len(fileInfo) > 0 {
				if jsonBytes, err := json.Marshal(fileInfo); err == nil {
					bodyContent = string(jsonBytes)
				}
			}
		}
	} else if strings.Contains(contentType, "application/json") {
		// JSON 请求
		if raw, exists := c.Get("raw_body"); exists {
			if str, ok := raw.(string); ok {
				if len(str) > MaxBodyLogSize {
					bodyContent = str[:MaxBodyLogSize] + "...[truncated]"
				} else {
					bodyContent = str
				}
			}
		}
	} else if c.Request.ContentLength > 0 {
		// 其他类型的请求体
		bodyContent = "[Not logged: Content-Type=" + contentType + "]"
	}

	// 构建最终的请求数据字符串
	result := strings.Join(parts, "&")

	// 如果有 body 内容（JSON 或文件信息），直接追加到字符串后面
	if bodyContent != "" {
		if result != "" {
			result += "&body=" + bodyContent
		} else {
			result = "body=" + bodyContent
		}
	}

	return result
}

// getBusinessStatus 从响应体中提取业务状态码和响应消息
func getBusinessStatus(body []byte) (int, string) {
	if len(body) == 0 {
		return 0, ""
	}
	var resp struct {
		Code    int    `json:"code"`
		Message string `json:"msg"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return 0, ""
	}
	return resp.Code, resp.Message
}

// WithOperation 操作类型装饰器
func WithOperation(actionType string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("operationType", actionType)
		c.Next()
	}
}

// CaptureRawBody 捕获请求体并存入 Context
func CaptureRawBody() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method == "GET" {
			c.Next()
			return
		}
		// 仅处理有 Body 的请求类型
		contentType := c.Request.Header.Get("Content-Type")
		if !strings.Contains(contentType, "application/json") {
			c.Next()
			return
		}

		// 如果 Body 为空，跳过
		if c.Request.Body == nil || c.Request.ContentLength == 0 {
			c.Next()
			return
		}

		// 读取 Body (建议也加个大小限制，防止超大包攻击)
		const maxCaptureSize = 64 * 1024 // 64KB
		bodyBytes, err := io.ReadAll(io.LimitReader(c.Request.Body, maxCaptureSize))

		if err == nil {
			// 存入 Context
			c.Set("raw_body", string(bodyBytes))

			// 重置 Body，确保后续 Handler (如 c.ShouldBindJSON) 能正常读取
			c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		} else {
			// 读取失败，可选：记录一个标记
			c.Set("raw_body_error", err.Error())
		}
		c.Next()
	}
}
