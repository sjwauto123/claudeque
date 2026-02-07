package response

type RolePermissionTree struct {
	Permissions []*MenuTreeNodeRole `json:"menu"` // 实际为 []*MenuTreeNode
	API         APITree             `json:"api"`
}

// MenuTreeNodeRole 角色权限树节点结构
type MenuTreeNodeRole struct {
	ID         int                 `json:"id"`
	Title      string              `json:"title"`
	Name       string              `json:"name,omitempty"`
	Type       string              `json:"type"` // catalogue/menu/permission
	Checked    bool                `json:"checked"`
	Permission []PermissionNode    `json:"permission,omitempty"`
	Children   []*MenuTreeNodeRole `json:"children,omitempty"`
}

// PermissionNode 权限节点结构
type PermissionNode struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	Category   string `json:"category"`
	Slug       string `json:"slug"`
	Type       string `json:"type"`
	Status     int    `json:"status"`
	HTTPMethod string `json:"http_method"`
	HTTPPath   string `json:"http_path"`
	Sort       int    `json:"sort"`
}

type APITree struct {
	Title    string     `json:"title"`
	Checked  bool       `json:"checked"`
	Children []*APINode `json:"children"`
}

type APINode struct {
	HTTPPath string `json:"http_path"`
	Checked  bool   `json:"checked"`
}
