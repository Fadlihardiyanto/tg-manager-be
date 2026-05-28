package mailer

import (
	"bytes"
	"fmt"
	"html/template"
	"net/smtp"
	"strings"
)

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

// renderOTPTemplate renders the OTP email HTML.
func renderOTPTemplate(code, purpose string) (string, error) {
	tmpl, err := template.New("otp").Parse(otpEmailTemplate)
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

// renderAdminVerificationTemplate renders the admin registration verification email HTML.
func renderAdminVerificationTemplate(name, verificationLink string) (string, error) {
	tmpl, err := template.New("admin_verify").Parse(adminVerificationEmailTemplate)
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

// renderTenantVerificationTemplate renders the tenant registration verification email HTML.
func renderTenantVerificationTemplate(name, verificationLink string) (string, error) {
	tmpl, err := template.New("tenant_verify").Parse(tenantVerificationEmailTemplate)
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

const otpEmailTemplate = `<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
</head>
<body style="margin:0;padding:0;background-color:#f4f4f7;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,sans-serif;">
    <table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="background-color:#f4f4f7;padding:40px 0;">
        <tr>
            <td align="center">
                <table role="presentation" width="480" cellspacing="0" cellpadding="0" style="background-color:#ffffff;border-radius:8px;box-shadow:0 2px 8px rgba(0,0,0,0.08);overflow:hidden;">
                    <!-- Header -->
                    <tr>
                        <td style="background-color:#1a1a2e;padding:24px 32px;">
                            <h1 style="margin:0;color:#ffffff;font-size:20px;font-weight:600;">TG-Manager</h1>
                        </td>
                    </tr>
                    <!-- Body -->
                    <tr>
                        <td style="padding:32px;">
                            <p style="margin:0 0 16px;color:#333;font-size:16px;">Your {{.Purpose}} code is:</p>
                            <div style="background-color:#f0f0f5;border-radius:8px;padding:20px;text-align:center;margin:0 0 24px;">
                                <span style="font-size:36px;font-weight:700;letter-spacing:8px;color:#1a1a2e;">{{.Code}}</span>
                            </div>
                            <p style="margin:0 0 8px;color:#666;font-size:14px;">This code expires in <strong>5 minutes</strong>.</p>
                            <p style="margin:0;color:#666;font-size:14px;">If you didn't request this code, please ignore this email or contact support if you have concerns.</p>
                        </td>
                    </tr>
                    <!-- Footer -->
                    <tr>
                        <td style="padding:16px 32px;background-color:#f9f9fb;border-top:1px solid #eee;">
                            <p style="margin:0;color:#999;font-size:12px;text-align:center;">This is an automated message from TG-Manager. Please do not reply.</p>
                        </td>
                    </tr>
                </table>
            </td>
        </tr>
    </table>
</body>
</html>`

const adminVerificationEmailTemplate = `<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
</head>
<body style="margin:0;padding:0;background-color:#f4f4f7;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,sans-serif;">
    <table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="background-color:#f4f4f7;padding:40px 0;">
        <tr>
            <td align="center">
                <table role="presentation" width="480" cellspacing="0" cellpadding="0" style="background-color:#ffffff;border-radius:8px;box-shadow:0 2px 8px rgba(0,0,0,0.08);overflow:hidden;">
                    <!-- Header -->
                    <tr>
                        <td style="background-color:#1a1a2e;padding:24px 32px;">
                            <h1 style="margin:0;color:#ffffff;font-size:20px;font-weight:600;">TG-Manager</h1>
                            <p style="margin:4px 0 0;color:#b0b0c0;font-size:14px;">Admin Portal</p>
                        </td>
                    </tr>
                    <!-- Body -->
                    <tr>
                        <td style="padding:32px;">
                            <h2 style="margin:0 0 16px;color:#1a1a2e;font-size:18px;font-weight:600;">Welcome, {{.Name}}!</h2>
                            <p style="margin:0 0 16px;color:#333;font-size:16px;">Thank you for registering as an admin. Please verify your email address to complete your registration and activate your account.</p>
                            <div style="text-align:center;margin:32px 0;">
                                <a href="{{.Link}}" style="display:inline-block;background-color:#4a90e2;color:#ffffff;padding:12px 32px;border-radius:6px;text-decoration:none;font-weight:600;font-size:16px;">Verify Email Address</a>
                            </div>
                            <p style="margin:24px 0 8px;color:#666;font-size:14px;">Or copy and paste this link in your browser:</p>
                            <p style="margin:0 0 24px;color:#4a90e2;font-size:12px;word-break:break-all;">{{.Link}}</p>
                            <hr style="border:none;border-top:1px solid #eee;margin:24px 0;">
                            <p style="margin:0 0 8px;color:#666;font-size:14px;">This link expires in <strong>24 hours</strong>.</p>
                            <p style="margin:0;color:#666;font-size:14px;">If you didn't create this account, please ignore this email or contact support if you have concerns.</p>
                        </td>
                    </tr>
                    <!-- Footer -->
                    <tr>
                        <td style="padding:16px 32px;background-color:#f9f9fb;border-top:1px solid #eee;">
                            <p style="margin:0;color:#999;font-size:12px;text-align:center;">This is an automated message from TG-Manager. Please do not reply.</p>
                        </td>
                    </tr>
                </table>
            </td>
        </tr>
    </table>
</body>
</html>`

const tenantVerificationEmailTemplate = `<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
</head>
<body style="margin:0;padding:0;background-color:#f4f4f7;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,sans-serif;">
    <table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="background-color:#f4f4f7;padding:40px 0;">
        <tr>
            <td align="center">
                <table role="presentation" width="480" cellspacing="0" cellpadding="0" style="background-color:#ffffff;border-radius:8px;box-shadow:0 2px 8px rgba(0,0,0,0.08);overflow:hidden;">
                    <!-- Header -->
                    <tr>
                        <td style="background-color:#1a1a2e;padding:24px 32px;">
                            <h1 style="margin:0;color:#ffffff;font-size:20px;font-weight:600;">TG-Manager</h1>
                            <p style="margin:4px 0 0;color:#b0b0c0;font-size:14px;">Business Portal</p>
                        </td>
                    </tr>
                    <!-- Body -->
                    <tr>
                        <td style="padding:32px;">
                            <h2 style="margin:0 0 16px;color:#1a1a2e;font-size:18px;font-weight:600;">Welcome, {{.Name}}!</h2>
                            <p style="margin:0 0 16px;color:#333;font-size:16px;">Thank you for registering your business. Please verify your email address to complete your registration and activate your account.</p>
                            <div style="text-align:center;margin:32px 0;">
                                <a href="{{.Link}}" style="display:inline-block;background-color:#4a90e2;color:#ffffff;padding:12px 32px;border-radius:6px;text-decoration:none;font-weight:600;font-size:16px;">Verify Email Address</a>
                            </div>
                            <p style="margin:24px 0 8px;color:#666;font-size:14px;">Or copy and paste this link in your browser:</p>
                            <p style="margin:0 0 24px;color:#4a90e2;font-size:12px;word-break:break-all;">{{.Link}}</p>
                            <hr style="border:none;border-top:1px solid #eee;margin:24px 0;">
                            <p style="margin:0 0 8px;color:#666;font-size:14px;">This link expires in <strong>24 hours</strong>.</p>
                            <p style="margin:0;color:#666;font-size:14px;">If you didn't create this account, please ignore this email or contact support if you have concerns.</p>
                        </td>
                    </tr>
                    <!-- Footer -->
                    <tr>
                        <td style="padding:16px 32px;background-color:#f9f9fb;border-top:1px solid #eee;">
                            <p style="margin:0;color:#999;font-size:12px;text-align:center;">This is an automated message from TG-Manager. Please do not reply.</p>
                        </td>
                    </tr>
                </table>
            </td>
        </tr>
    </table>
</body>
</html>`
