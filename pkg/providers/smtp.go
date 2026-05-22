package providers

import (
	"context"
	"fmt"
	"net/smtp"
)

// SMTPProvider sends emails via SMTP
type SMTPProvider struct {
	Host     string
	Port     int
	Username string
	Password string
	FromAddr string
}

// NewSMTPProvider factory function
func NewSMTPProvider(config map[string]interface{}) (Provider, error) {
	p := &SMTPProvider{}

	if err := p.parseConfig(config); err != nil {
		return nil, err
	}

	return p, nil
}

func (p *SMTPProvider) parseConfig(config map[string]interface{}) error {
	var ok bool

	p.Host, ok = config["host"].(string)
	if !ok || p.Host == "" {
		return fmt.Errorf("%w: missing 'host'", ErrInvalidConfig)
	}

	port, ok := config["port"].(float64)
	if !ok {
		port = 587
	}
	p.Port = int(port)

	p.Username, ok = config["username"].(string)
	if !ok || p.Username == "" {
		return fmt.Errorf("%w: missing 'username'", ErrInvalidConfig)
	}

	p.Password, ok = config["password"].(string)
	if !ok || p.Password == "" {
		return fmt.Errorf("%w: missing 'password'", ErrInvalidConfig)
	}

	p.FromAddr, ok = config["from"].(string)
	if !ok || p.FromAddr == "" {
		return fmt.Errorf("%w: missing 'from'", ErrInvalidConfig)
	}

	return nil
}

func (p *SMTPProvider) Name() string {
	return "smtp"
}

func (p *SMTPProvider) Initialize(config map[string]interface{}) error {
	return p.parseConfig(config)
}

func (p *SMTPProvider) Send(ctx context.Context, req *NotificationRequest) (*NotificationResponse, error) {
	addr := fmt.Sprintf("%s:%d", p.Host, p.Port)

	auth := smtp.PlainAuth("", p.Username, p.Password, p.Host)

	message := fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s",
		p.FromAddr,
		req.Recipient,
		req.Subject,
		req.Content,
	)

	err := smtp.SendMail(addr, auth, p.FromAddr, []string{req.Recipient}, []byte(message))

	if err != nil {
		return &NotificationResponse{
			Success:      false,
			ErrorMessage: err.Error(),
		}, nil
	}

	return &NotificationResponse{
		Success:     true,
		ProviderRef: "smtp-" + req.ID,
	}, nil
}

func (p *SMTPProvider) Health() error {
	addr := fmt.Sprintf("%s:%d", p.Host, p.Port)
	auth := smtp.PlainAuth("", p.Username, p.Password, p.Host)

	c, err := smtp.Dial(addr)
	if err != nil {
		return err
	}
	defer c.Close()

	if err = c.StartTLS(nil); err != nil {
		return err
	}

	if err = c.Auth(auth); err != nil {
		return err
	}

	return c.Quit()
}

func init() {
	Register("smtp", NewSMTPProvider)
}
