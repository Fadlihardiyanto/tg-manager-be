package mailer

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"net/smtp"
	"strings"
)

//go:embed templates/*.html
var templatesFS embed.FS

// Config holds SMTP connection settings.
type Config struct {
	Host      string
	Port      int
	Username  string
	Password  string
	FromEmail string
	FromName  string
}

// Mailer sends emails via SMTP.
type Mailer struct {
	config Config
}

// New creates a new Mailer instance.
func New(cfg Config) *Mailer {
	return &Mailer{config: cfg}
}

// SendOTP sends a 6-digit OTP code to the specified email address.
func (m *Mailer) SendOTP(to, code, purpose string) error {
	subject := "Your Verification Code"
	if purpose == "admin_login_2fa" {
		subject = "TG-Manager Admin - Login Verification Code"
	}

	body, err := renderOTPTemplate(code, purpose)
	if err != nil {
		return fmt.Errorf("mailer: render template: %w", err)
	}

	return m.send(to, subject, body)
}

// SendAdminVerificationEmail sends a registration verification email to a new admin user.
func (m *Mailer) SendAdminVerificationEmail(to, name, verificationLink string) error {
	subject := "Verify Your Email - TG-Manager Admin"

	body, err := renderAdminVerificationTemplate(name, verificationLink)
	if err != nil {
		return fmt.Errorf("mailer: render template: %w", err)
	}

	return m.send(to, subject, body)
}

// SendTenantVerificationEmail sends a registration verification email to a new tenant user.
func (m *Mailer) SendTenantVerificationEmail(to, name, verificationLink string) error {
	subject := "Verify Your Email - TG-Manager"

	body, err := renderTenantVerificationTemplate(name, verificationLink)
	if err != nil {
		return fmt.Errorf("mailer: render template: %w", err)
	}

	return m.send(to, subject, body)
}

// send dispatches an email via SMTP.
func (m *Mailer) send(to, subject, htmlBody string) error {
	from := m.config.FromEmail
	if m.config.FromName != "" {
		from = fmt.Sprintf("%s <%s>", m.config.FromName, m.config.FromEmail)
	}

	// Build MIME message
	var msg strings.Builder
	msg.WriteString(fmt.Sprintf("From: %s\r\n", from))
	msg.WriteString(fmt.Sprintf("To: %s\r\n", to))
	msg.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	msg.WriteString("MIME-Version: 1.0\r\n")
	msg.WriteString("Content-Type: text/html; charset=\"UTF-8\"\r\n")
	msg.WriteString("\r\n")
	msg.WriteString(htmlBody)

	addr := fmt.Sprintf("%s:%d", m.config.Host, m.config.Port)
	auth := smtp.PlainAuth("", m.config.Username, m.config.Password, m.config.Host)

	return smtp.SendMail(addr, auth, m.config.FromEmail, []string{to}, []byte(msg.String()))
}

func renderOTPTemplate(code, purpose string) (string, error) {
	tmpl, err := template.ParseFS(templatesFS, "templates/otp.html")
	if err != nil {
		return "", err
	}

	purposeText := "verification"
	if purpose == "admin_login_2fa" {
		purposeText = "login verification"
	} else if purpose == "admin_2fa_setup" {
		purposeText = "2FA setup"
	}

	data := map[string]string{
		"Code":    code,
		"Purpose": purposeText,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func renderAdminVerificationTemplate(name, verificationLink string) (string, error) {
	tmpl, err := template.ParseFS(templatesFS, "templates/verification.html")
	if err != nil {
		return "", err
	}

	data := map[string]string{
		"Name": name,
		"Link": verificationLink,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func renderTenantVerificationTemplate(name, verificationLink string) (string, error) {
	tmpl, err := template.ParseFS(templatesFS, "templates/verification.html")
	if err != nil {
		return "", err
	}

	data := map[string]string{
		"Name": name,
		"Link": verificationLink,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}


