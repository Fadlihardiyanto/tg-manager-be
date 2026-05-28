package model

// EmailNotificationType defines supported email task types.
type EmailNotificationType string

const (
	EmailNotificationOTP               EmailNotificationType = "otp"
	EmailNotificationAdminVerification EmailNotificationType = "admin_verification"
	EmailNotificationTenantVerification EmailNotificationType = "tenant_verification"
)

// EmailNotificationPayload represents a queued email task.
type EmailNotificationPayload struct {
	Type             EmailNotificationType `json:"type"`
	To               string                `json:"to"`
	Code             string                `json:"code,omitempty"`
	Purpose          string                `json:"purpose,omitempty"`
	Name             string                `json:"name,omitempty"`
	VerificationLink string                `json:"verification_link,omitempty"`
}
