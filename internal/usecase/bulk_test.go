package usecase

import (
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestRunBulkDelete(t *testing.T) {
	id1, id2, id3 := uuid.New(), uuid.New(), uuid.New()

	t.Run("all succeed", func(t *testing.T) {
		result := RunBulkDelete([]uuid.UUID{id1, id2, id3}, func(id uuid.UUID) error {
			return nil
		})
		if result.Deleted != 3 {
			t.Errorf("Deleted = %d, want 3", result.Deleted)
		}
		if len(result.Failed) != 0 {
			t.Errorf("Failed = %v, want none", result.Failed)
		}
	})

	t.Run("best effort with failures", func(t *testing.T) {
		result := RunBulkDelete([]uuid.UUID{id1, id2, id3}, func(id uuid.UUID) error {
			if id == id2 {
				return errors.New("not found")
			}
			return nil
		})
		if result.Deleted != 2 {
			t.Errorf("Deleted = %d, want 2", result.Deleted)
		}
		if len(result.Failed) != 1 {
			t.Fatalf("Failed = %v, want 1 entry", result.Failed)
		}
		if result.Failed[0].ID != id2 {
			t.Errorf("failed id = %v, want %v", result.Failed[0].ID, id2)
		}
		if result.Failed[0].Error != "not found" {
			t.Errorf("failed error = %q, want %q", result.Failed[0].Error, "not found")
		}
	})

	t.Run("empty ids", func(t *testing.T) {
		result := RunBulkDelete(nil, func(id uuid.UUID) error { return nil })
		if result.Deleted != 0 || len(result.Failed) != 0 {
			t.Errorf("expected empty result, got %+v", result)
		}
	})
}
