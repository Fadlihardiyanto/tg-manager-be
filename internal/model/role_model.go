package model

import (
	"time"

	"github.com/google/uuid"
)

// Role Models
type RoleCreateRequest struct {
	Name        string `json:"name" validate:"required,min=3"`
	DisplayName string `json:"display_name" validate:"required,min=3"`
	Description string `json:"description" validate:"omitempty"`
}

type RoleResponse struct {
	ID          uuid.UUID            `json:"id"`
	Name        string               `json:"name"`
	DisplayName string               `json:"display_name"`
	Description string               `json:"description"`
	Permissions []PermissionResponse `json:"permissions,omitempty"`
	CreatedAt   time.Time            `json:"created_at"`
}

// Permission Models
type PermissionCreateRequest struct {
	Name        string `json:"name" validate:"required,min=3"`
	Module      string `json:"module" validate:"required,oneof=packages groups bots orders analytics"`
	Action      string `json:"action" validate:"required,oneof=create read update delete"`
	Description string `json:"description" validate:"omitempty"`
}

type PermissionResponse struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Module      string    `json:"module"`
	Action      string    `json:"action"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}

// Role Permission Assignment
type RolePermissionAssignRequest struct {
	PermissionIDs []uuid.UUID `json:"permission_ids" validate:"required"`
}
