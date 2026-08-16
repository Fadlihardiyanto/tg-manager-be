package usecase

import "errors"

// Sentinel errors for Midtrans webhook handling.
// Terminal outcomes → controller returns 200 (stop Midtrans retries).
// Everything else (plain errors) → controller returns 503 (let Midtrans retry).
var (
	ErrWebhookInvalidSignature = errors.New("webhook: invalid signature")
	ErrWebhookNotFound         = errors.New("webhook: target not found")
	ErrWebhookAmountMismatch   = errors.New("webhook: gross amount mismatch")
)
