package response

type PermissionResponse struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	Category   string `json:"category"`
	Slug       string `json:"slug"`
	Type       string `json:"type"`
	Status     int    `json:"status"`
	HttpMethod string `json:"http_method"`
	HttpPath   string `json:"http_path"`
	Sort       int    `json:"sort"`
}

type ApiPermissionRes struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Slug    string `json:"slug"`
	Checked bool   `json:"checked"`
}

type ApiPermissionGroupRes struct {
	Category string              `json:"category"`
	List     []*ApiPermissionRes `json:"list"`
}
