package converter

import (
	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/model"
)

// PackageToResponse converts Package entity to PackageResponse model
func PackageToResponse(pkg *entity.Package) *model.PackageResponse {
	if pkg == nil {
		return nil
	}

	var groupResponses []model.GroupResponse
	if len(pkg.Groups) > 0 {
		groupResponses = make([]model.GroupResponse, len(pkg.Groups))
		for i, g := range pkg.Groups {
			groupResponses[i] = *GroupToResponse(&g)
		}
	}

	return &model.PackageResponse{
		ID:           pkg.ID,
		ClientID:     pkg.ClientID,
		Name:         pkg.Name,
		Description:  pkg.Description,
		Price:        pkg.Price,
		DurationDays: pkg.DurationDays,
		IsAllAccess:  pkg.IsAllAccess,
		IsActive:     pkg.IsActive,
		CreatedAt:    pkg.CreatedAt,
		UpdatedAt:    pkg.UpdatedAt,
		Groups:       groupResponses,
	}
}

// PackagesToResponse converts multiple Package entities to PackageResponse models
func PackagesToResponse(packages []entity.Package) []model.PackageResponse {
	if len(packages) == 0 {
		return []model.PackageResponse{}
	}

	responses := make([]model.PackageResponse, len(packages))
	for i, pkg := range packages {
		responses[i] = *PackageToResponse(&pkg)
	}
	return responses
}
