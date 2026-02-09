package response

type MenuNode struct {
	ID       int        `json:"id"`
	ParentID int        `json:"parent_id"`
	Title    string     `json:"title"`
	Status   int        `json:"status"`
	Type     string     `json:"type"`
	Icon     string     `json:"icon"`
	URI      string     `json:"uri"`
	Sort     int        `json:"sort"`
	Children []MenuNode `json:"children"`
}
