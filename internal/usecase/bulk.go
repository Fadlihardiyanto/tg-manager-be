package usecase

import (
	"github.com/Fadlihardiyanto/telegram-management-app/internal/model"
	"github.com/google/uuid"
)

// RunBulkDelete performs a best-effort bulk delete: each id is deleted via
// deleteFn independently, and any failures are collected per id. Ids are
// processed sequentially — side effects (S3, cache, Telegram API, quota checks)
// behave exactly like the single-item delete.
func RunBulkDelete(ids []uuid.UUID, deleteFn func(uuid.UUID) error) model.BulkDeleteResult {
	result := model.BulkDeleteResult{Deleted: 0, Failed: []model.BulkDeleteFailure{}}
	for _, id := range ids {
		if err := deleteFn(id); err != nil {
			result.Failed = append(result.Failed, model.BulkDeleteFailure{ID: id, Error: err.Error()})
			continue
		}
		result.Deleted++
	}
	if len(result.Failed) == 0 {
		result.Failed = nil
	}
	return result
}
