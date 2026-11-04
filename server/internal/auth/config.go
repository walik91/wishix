package auth

import (
	"errors"
	"net"
	"net/mail"
	"net/url"
	"os"
	"strings"
)

type Config struct {
	Origin         string
	SecureCookie   bool
	Secret         string
	TrustedProxies []*net.IPNet
	SMTPHost       string
	SMTPPort       string
	SMTPFrom       string
	SMTPUsername   string
	SMTPPassword   string
	SMTPSecurity   string
}

func LoadConfig() (*Config, error) {
	c := &Config{Origin: os.Getenv("APP_ORIGIN"), Secret: os.Getenv("AUTH_SECRET"), SMTPHost: os.Getenv("SMTP_HOST"), SMTPPort: os.Getenv("SMTP_PORT"), SMTPFrom: os.Getenv("SMTP_FROM"), SMTPUsername: os.Getenv("SMTP_USERNAME"), SMTPPassword: os.Getenv("SMTP_PASSWORD"), SMTPSecurity: os.Getenv("SMTP_SECURITY")}
	origin, err := url.Parse(c.Origin)
	if err != nil || origin.Host == "" || origin.User != nil || origin.RawQuery != "" || origin.Fragment != "" || origin.Path != "" || (origin.Scheme != "http" && origin.Scheme != "https") {
		return nil, errors.New("APP_ORIGIN must be an http(s) origin without a path")
	}
	c.SecureCookie = origin.Scheme == "https"
	if len(c.Secret) < 32 {
		return nil, errors.New("AUTH_SECRET must contain at least 32 random characters")
	}
	if c.SMTPHost == "" || c.SMTPPort == "" {
		return nil, errors.New("SMTP_HOST and SMTP_PORT are required")
	}
	from, err := mail.ParseAddress(c.SMTPFrom)
	if err != nil || from.Address != c.SMTPFrom {
		return nil, errors.New("SMTP_FROM must be an email address")
	}
	if c.SMTPSecurity != "none" && c.SMTPSecurity != "starttls" && c.SMTPSecurity != "tls" {
		return nil, errors.New("SMTP_SECURITY must be none, starttls or tls")
	}
	if (c.SMTPUsername == "") != (c.SMTPPassword == "") {
		return nil, errors.New("SMTP_USERNAME and SMTP_PASSWORD must be configured together")
	}
	development := os.Getenv("APP_ENV") == "development"
	if !development && (!c.SecureCookie || c.SMTPSecurity == "none") {
		return nil, errors.New("production requires HTTPS APP_ORIGIN and encrypted SMTP")
	}
	if c.SMTPSecurity == "none" && c.SMTPUsername != "" {
		return nil, errors.New("SMTP authentication requires TLS")
	}
	for _, cidr := range strings.Split(os.Getenv("AUTH_TRUSTED_PROXY_CIDRS"), ",") {
		if strings.TrimSpace(cidr) == "" {
			continue
		}
		_, network, err := net.ParseCIDR(strings.TrimSpace(cidr))
		if err != nil {
			return nil, errors.New("invalid AUTH_TRUSTED_PROXY_CIDRS")
		}
		c.TrustedProxies = append(c.TrustedProxies, network)
	}
	return c, nil
}
