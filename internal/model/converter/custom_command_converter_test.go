package converter

import (
	"testing"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	"github.com/google/uuid"
	"github.com/lib/pq"
)

func TestCustomCommandToResponseConvertsUUIDArrays(t *testing.T) {
	packageID := uuid.New()
	groupID := uuid.New()

	got := CustomCommandToResponse(&entity.CustomCommand{
		PackageIDs: pq.StringArray{packageID.String()},
		GroupIDs:   pq.StringArray{groupID.String()},
	})

	if len(got.PackageIDs) != 1 || got.PackageIDs[0] != packageID {
		t.Fatalf("package_ids = %v, want [%s]", got.PackageIDs, packageID)
	}
	if len(got.GroupIDs) != 1 || got.GroupIDs[0] != groupID {
		t.Fatalf("group_ids = %v, want [%s]", got.GroupIDs, groupID)
	}
}
