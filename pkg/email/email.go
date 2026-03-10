package email

import (
	"cloudque/pkg/config"

	"cloudque/pkg/logger"
	"go.uber.org/zap"
	"gopkg.in/gomail.v2"
)

// SendEmail 发送邮件
func SendEmail(to string, subject string, body string) error {
	cfg := config.Get().Email
	m := gomail.NewMessage()
	m.SetHeader("From", cfg.Username)
	m.SetHeader("To", to)
	m.SetHeader("Subject", subject)
	m.SetBody("text/html", body)

	d := gomail.NewDialer(cfg.Host, cfg.Port, cfg.Username, cfg.Password)
	//return d.DialAndSend(m)
	err := d.DialAndSend(m)
	logger.Error("发送验证码错误:", zap.Error(err))
	return err
}
