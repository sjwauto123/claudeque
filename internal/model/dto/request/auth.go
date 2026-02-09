package request

// SendEmailCodeRequest 发送邮箱验证码请求
type SendEmailCodeRequest struct {
	Email string `json:"email" binding:"required,email"`
}

// RefreshTokenRequest 刷新 Token 请求
type RefreshTokenRequest struct {
	Token string `json:"token" binding:"required"`
}
