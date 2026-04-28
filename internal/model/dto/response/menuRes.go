package response

// MenuResponse 菜单响应结构
type MenuResponse struct {
	ID        int    `json:"id"`
	ParentID  int    `json:"parent_id,omitempty"`
	Title     string `json:"title"`
	Type      string `json:"type"`   // catalogue/menu
	Status    int    `json:"status"` // 0=停用, 1=启用
	Icon      string `json:"icon,omitempty"`
	URI       string `json:"uri"`
	Sort      int    `json:"sort"`
	CreatedAt string `json:"created_at,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

// MenuTreeNode 返回菜单树结构
type MenuTreeNode struct {
	ID       int             `json:"id"`
	ParentID int             `json:"parent_id"`
	Title    string          `json:"title"`
	Type     string          `json:"type"`   // catalogue/menu
	Status   int             `json:"status"` // 0=停用, 1=启用
	Icon     string          `json:"icon,omitempty"`
	URI      string          `json:"uri"`
	Sort     int             `json:"sort"`
	Children []*MenuTreeNode `json:"children,omitempty"`
}

type MenuNodeResponse struct {
	ID       int                 `json:"id"`
	Title    string              `json:"title"`
	Checked  bool                `json:"checked"`
	Children []*MenuNodeResponse `json:"children,omitempty"`
}

// MenuNode 返回登录用户的菜单树结构
//type MenuNode struct {
//	ID       int        `json:"id"`
//	ParentID int        `json:"parent_id"`
//	Title    string     `json:"title"`
//	Status   int        `json:"status"`
//	Type     string     `json:"type"`
//	Icon     string     `json:"icon"`
//	URI      string     `json:"uri"`
//	Sort     int        `json:"sort"`
//	Children []MenuNode `json:"children"`
//}
