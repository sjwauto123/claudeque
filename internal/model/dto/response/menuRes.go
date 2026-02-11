package response

// MenuResponse 菜单响应结构
type MenuResponse struct {
	ID        int    `json:"id"`
	ParentID  int    `json:"parent_id,omitempty"`
	Title     string `json:"title"`
	Type      string `json:"type"`   // catalogue/menu/button
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
	Type     string          `json:"type"`   // catalogue/menu/button
	Status   int             `json:"status"` // 0=停用, 1=启用
	Icon     string          `json:"icon,omitempty"`
	URI      string          `json:"uri"`
	Sort     int             `json:"sort"`
	Children []*MenuTreeNode `json:"children,omitempty"`
}

type MenuNodeRes struct {
	ID       int            `json:"id"`
	Title    string         `json:"title"`
	Checked  bool           `json:"checked"`
	Children []*MenuNodeRes `json:"children,omitempty"`
}
