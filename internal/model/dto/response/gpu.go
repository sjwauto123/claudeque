package response

type GpuSpec struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`   // gpu-0
	Type   string `json:"type"`   // NVIDIA A100
	Memory int    `json:"memory"` // MB
	Gb     int    `json:"gb"`     // GB
}

type Gpus struct {
	List  any `json:"list"`
	Total int `json:"total"`
}
