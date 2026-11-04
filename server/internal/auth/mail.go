package auth

import (
	"context"
	"crypto/tls"
	"fmt"
	"mime"
	"net"
	"net/smtp"
	"time"
)

type Mailer struct{ config *Config }

func NewMailer(config *Config) *Mailer { return &Mailer{config: config} }

// SendCode never logs messages or credentials. SMTP is outside retryable DB work.
func (m *Mailer) SendCode(ctx context.Context, email, locale, purpose, code string) error {
	subject, body := codeMessage(locale, purpose, code)
	c := m.config
	deadline := time.Now().Add(5 * time.Second)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	dialer := &net.Dialer{Deadline: deadline}
	address := net.JoinHostPort(c.SMTPHost, c.SMTPPort)
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return ErrDelivery
	}
	defer conn.Close()
	if err := conn.SetDeadline(deadline); err != nil {
		return ErrDelivery
	}
	tlsConfig := &tls.Config{ServerName: c.SMTPHost, MinVersion: tls.VersionTLS12}
	if c.SMTPSecurity == "tls" {
		secured := tls.Client(conn, tlsConfig)
		if err := secured.HandshakeContext(ctx); err != nil {
			return ErrDelivery
		}
		conn = secured
	}
	client, err := smtp.NewClient(conn, c.SMTPHost)
	if err != nil {
		return ErrDelivery
	}
	defer client.Close()
	if c.SMTPSecurity == "starttls" {
		if err := client.StartTLS(tlsConfig); err != nil {
			return ErrDelivery
		}
	}
	if c.SMTPUsername != "" {
		if err := client.Auth(smtp.PlainAuth("", c.SMTPUsername, c.SMTPPassword, c.SMTPHost)); err != nil {
			return ErrDelivery
		}
	}
	if err := client.Mail(c.SMTPFrom); err != nil {
		return ErrDelivery
	}
	if err := client.Rcpt(email); err != nil {
		return ErrDelivery
	}
	writer, err := client.Data()
	if err != nil {
		return ErrDelivery
	}
	message := fmt.Sprintf("From: Wishix <%s>\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\nContent-Transfer-Encoding: 8bit\r\n\r\n%s\r\n", c.SMTPFrom, email, mime.QEncoding.Encode("UTF-8", subject), body)
	if _, err := writer.Write([]byte(message)); err != nil {
		return ErrDelivery
	}
	if err := writer.Close(); err != nil {
		return ErrDelivery
	}
	// DATA acknowledgement is the delivery result; QUIT failure must not hide it.
	_ = client.Quit()
	return nil
}

func codeMessage(locale, purpose, code string) (string, string) {
	if locale == "ru" {
		action := "подтверждения email"
		if purpose == "reset" {
			action = "восстановления пароля"
		}
		return "Wishix — код " + action, fmt.Sprintf("Код %s: %s\r\n\r\nКод действует 10 минут. Никому его не сообщайте. Если вы не запрашивали код, просто проигнорируйте это письмо.", action, code)
	}
	if locale == "ro" {
		action := "confirmarea adresei de email"
		if purpose == "reset" {
			action = "resetarea parolei"
		}
		return "Wishix — cod pentru " + action, fmt.Sprintf("Cod pentru %s: %s\r\n\r\nCodul este valabil 10 minute. Nu îl divulgați nimănui. Dacă nu ați solicitat acest cod, ignorați acest mesaj.", action, code)
	}
	action := "email verification"
	if purpose == "reset" {
		action = "password reset"
	}
	return "Wishix — " + action + " code", fmt.Sprintf("Your %s code: %s\r\n\r\nThis code expires in 10 minutes. Do not share it. If you did not request it, ignore this email.", action, code)
}
