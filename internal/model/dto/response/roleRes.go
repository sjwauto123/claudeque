package response

// RolePermissionNodeRes 角色权限节点结构
type RolePermissionNodeRes struct {
	ID         int                      `json:"id"`
	Title      string                   `json:"title"`
	Checked    bool                     `json:"checked"`
	Permission []*PermissionNodeRes     `json:"permission"`
	Children   []*RolePermissionNodeRes `json:"children"`
}

// PermissionNodeRes 权限节点结构
type PermissionNodeRes struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Slug    string `json:"slug"`
	Checked bool   `json:"checked"`
}
