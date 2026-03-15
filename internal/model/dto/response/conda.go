package response

type CondaEnv struct {
	Name      string `json:"name"`
	Prefix    string `json:"prefix"`
	IsDefault bool   `json:"is_default"`
}
