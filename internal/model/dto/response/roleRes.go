package response

type RolePermissionTree struct {
	Permissions []*MenuTreeNode `json:"data_permissions"` // 实际为 []*MenuTreeNode
	API         APITree         `json:"data_api"`
}

type MenuTreeNode struct {
	ID       int             `json:"id"`
	Title    string          `json:"title"`
	Name     string          `json:"name,omitempty"`
	Type     string          `json:"type"` // catalogue/menu/permission
	Checked  bool            `json:"checked"`
	Children []*MenuTreeNode `json:"children,omitempty"`
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
