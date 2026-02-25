package response

// APIResponse API响应结构
type APIResponse struct {
	ID          int    `json:"id"`          // 权限ID，主键
	Name        string `json:"name"`        // 权限名称
	Category    string `json:"category"`    // API类别也就是菜单名称
	Description string `json:"description"` // 权限API详情
	Slug        string `json:"slug"`        // 权限唯一标识，代码中使用
	Status      int    `json:"status"`      // 权限状态 0-停用 1-启用
	HTTPMethod  string `json:"http_method"` // API请求方法（GET/POST等）
	HTTPPath    string `json:"http_path"`   // API路径，支持通配符
}
