package model

import (
	"time"

	"github.com/google/uuid"
)

type MigrationMemberCSVRow struct {
	Username  string `json:"username" validate:"required"`
	ExpiredAt string `json:"expired_at" validate:"required"`
}

type MigrationMemberImportRequest struct {
	PackageID string                  `json:"package_id" validate:"required,uuid"`
	Members   []MigrationMemberCSVRow `json:"members" validate:"required,min=1,max=1000,dive"`
}

type MigrationMemberImportResponse struct {
	TotalSubmitted int                             `json:"total_submitted"`
	Imported       int                             `json:"imported"`
	Skipped        int                             `json:"skipped"`
	Errors         []MigrationMemberImportRowError `json:"errors,omitempty"`
}

type MigrationMemberImportRowError struct {
	Row      int    `json:"row"`
	Username string `json:"username"`
	Error    string `json:"error"`
}

type MigrationMemberResponse struct {
	ID        uuid.UUID  `json:"id"`
	ClientID  uuid.UUID  `json:"client_id"`
	PackageID uuid.UUID  `json:"package_id"`
	Username  string     `json:"username"`
	ExpiredAt time.Time  `json:"expired_at"`
	Status    string     `json:"status"`
	CreatedAt time.Time  `json:"created_at"`
	ClaimedAt *time.Time `json:"claimed_at,omitempty"`
}

type MigrationMemberFilterRequest struct {
	Page      int       `json:"page"`
	Limit     int       `json:"limit"`
	Status    string    `json:"status"`
	Search    string    `json:"search"`
	PackageID uuid.UUID `json:"package_id"`
}
