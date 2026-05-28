package mailer

// Sender defines the outbound email interface used by usecases.
type Sender interface {
	SendOTP(to, code, purpose string) error
	SendAdminVerificationEmail(to, name, verificationLink string) error
	SendTenantVerificationEmail(to, name, verificationLink string) error
}
