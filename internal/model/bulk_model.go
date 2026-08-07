package model

import "github.com/google/uuid"

// BulkDeleteRequest is the shared body for best-effort bulk delete endpoints.
type BulkDeleteRequest struct {
	IDs []uuid.UUID `json:"ids" validate:"required,min=1,max=100,dive,uuid"`
}

// BulkDeleteResult reports how many items were deleted and which failed.
type BulkDeleteResult struct {
	Deleted int                `json:"deleted"`
	Failed  []BulkDeleteFailure `json:"failed,omitempty"`
}

type BulkDeleteFailure struct {
	ID    uuid.UUID `json:"id"`
	Error string    `json:"error"`
}

// BulkMemberKickItem pairs a member with the specific subscription to cancel.
type BulkMemberKickItem struct {
	MemberID       uuid.UUID `json:"member_id" validate:"required,uuid"`
	SubscriptionID uuid.UUID `json:"subscription_id" validate:"required,uuid"`
}

// BulkMemberKickRequest is the body for best-effort bulk selective kick.
type BulkMemberKickRequest struct {
	Items []BulkMemberKickItem `json:"items" validate:"required,min=1,max=100,dive"`
}
