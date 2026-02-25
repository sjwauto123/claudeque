package middleware

import (
	"bytes"
	"cloudque/internal/model/entity"
	"cloudque/internal/service"
	"cloudque/pkg/logger"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// WithOperation 操作类型装饰器
func WithOperation(actionType string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("operationType", actionType)
		c.Next()
	}
}

// UserOperationLogs 操作日志中间件
func UserOperationLogs(userLogService service.UserOperationLogService) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. 捕获请求数据（按优先级获取）
		// 优先级1: 获取 URL 查询参数（如 /api/users?id=1&name=test）
		requestData := getDataFormUrl(c)

		// 优先级2: 获取 URL 路径参数（如 /api/users/123 中的 123）

		requestData = getDataFormPath(c, requestData)

		// 优先级3: 获取请求体数据（POST/PUT 等请求）

		requestData = getDataFormBody(c, requestData)

		// 2. 记录请求开始时间
		startTime := time.Now()

		// 3. 包装响应写入器，捕获状态码
		blw := &bodyLogWriter{body: bytes.NewBufferString(""), ResponseWriter: c.Writer}
		c.Writer = blw

		// 4. 处理请求
		c.Next()

		// 5. 获取用户信息
		username := GetUsername(c)
		if username == "" {
			return
		}

		// 6. 构建操作日志
		// 从上下文中获取操作类型（由装饰器设置）
		actionType := ""
		opType, exists := c.Get("operationType")
		if !exists {
			return
		}
		if typeStr, ok := opType.(string); ok {
			actionType = typeStr
		}
		// 获取业务状态码和响应消息（优先从响应体中提取）
		status := getBusinessStatusCode(blw.Body())

		// 如果没有提取到业务状态码，使用 HTTP 状态码
		if status == 0 {
			status = blw.Status()
		}

		log := &entity.UserOperationLog{
			Username:    username,
			Method:      c.Request.Method,
			Path:        c.Request.URL.Path,
			ActionType:  actionType,
			RequestData: requestData,
			Status:      status,
			CreatedAt:   startTime,
			UpdatedAt:   time.Now(),
		}

		// 7. 异步保存日志
		go func() {
			if err := userLogService.CreateLog(log); err != nil {
				// 日志保存失败时，输出到标准错误
				logger.Info("日志保存失败")
			}
		}()
	}
}

// bodyLogWriter 包装 gin.ResponseWriter 以捕获响应状态码和响应体
type bodyLogWriter struct {
	gin.ResponseWriter
	body       *bytes.Buffer
	statusCode int
}

func (w *bodyLogWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

func (w *bodyLogWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *bodyLogWriter) Status() int {
	if w.statusCode == 0 {
		return 200
	}
	return w.statusCode
}

func (w *bodyLogWriter) Body() []byte {
	return w.body.Bytes()
}

// getBusinessStatusCode 从响应体中提取业务状态码和响应消息
func getBusinessStatusCode(body []byte) int {
	if len(body) == 0 {
		return 0
	}
	// 尝试解析响应体为统一响应结构
	var resp struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return 0
	}
	return resp.Code
}

func getDataFormUrl(c *gin.Context) string {
	query := c.Request.URL.Query()
	if len(query) > 0 {
		requestData := query.Encode()
		return requestData
	}
	return ""
}

func getDataFormPath(c *gin.Context, requestData string) string {

	if len(c.Params) > 0 {
		params := make(map[string]string)
		for _, param := range c.Params {
			params[param.Key] = param.Value
		}
		// 如果已有查询参数，追加路径参数
		if requestData != "" {
			requestData += "&"
		}
		// 将路径参数转换为查询字符串格式
		for key, value := range params {
			requestData += key + "=" + value + "&"
		}
		// 去除最后的 &
		requestData = strings.TrimSuffix(requestData, "&")
	}
	return requestData

}

func getDataFormBody(c *gin.Context, requestData string) string {
	if requestData == "" && c.Request.Method != http.MethodGet {
		if c.Request.Body != nil && c.Request.ContentLength > 0 {
			requestBody, err := io.ReadAll(c.Request.Body)
			if err == nil {
				requestData = string(requestBody)
			}
			// 重置请求体
			c.Request.Body = io.NopCloser(bytes.NewBuffer(requestBody))
		}

		// 如果请求体为空，尝试从表单获取
		if requestData == "" {
			if err := c.Request.ParseForm(); err == nil {
				if len(c.Request.PostForm) > 0 {
					requestData = c.Request.PostForm.Encode()
				}
			}
		}
	}
	return requestData
}
