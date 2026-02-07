package response

// MenuResponse 菜单响应结构
type MenuResponse struct {
	ID        int    `json:"id"`
	ParentID  *int   `json:"parent_id,omitempty"`
	Title     string `json:"title"`
	Type      string `json:"type"`   // catalogue/menu/permission
	Status    int    `json:"status"` // 0=停用, 1=启用
	Icon      string `json:"icon,omitempty"`
	URI       string `json:"uri"`
	Sort      int    `json:"sort"`
	CreatedAt string `json:"created_at,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

// MenuTreeNode 菜单树节点结构（用于树形展示）
type MenuTreeNode struct {
	ID       int             `json:"id"`
	Title    string          `json:"title"`
	Type     string          `json:"type"`   // catalogue/menu/permission
	Status   int             `json:"status"` // 0=停用, 1=启用
	Icon     string          `json:"icon,omitempty"`
	URI      string          `json:"uri"`
	Sort     int             `json:"sort"`
	Children []*MenuTreeNode `json:"children,omitempty"`
}

// MenuWithPermissionResponse 带权限信息的菜单响应
type MenuWithPermissionResponse struct {
	MenuResponse
	Permission *PermissionResponse `json:"permission,omitempty"`
}

// PermissionResponse 权限响应结构
type PermissionResponse struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Slug     string `json:"slug"`
	Type     string `json:"type"`   // catalogue/menu/permission
	Status   int    `json:"status"` // 0=停用, 1=启用
	HTTPPath string `json:"http_path,omitempty"`
	Sort     int    `json:"sort"`
	Category string `json:"category,omitempty"`
}
