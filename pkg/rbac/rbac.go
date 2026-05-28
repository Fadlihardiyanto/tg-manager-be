package rbac

import "slices"

// HasPermission cek apakah slice permissions mengandung permission yang dibutuhkan.
// Dipakai di usecase (layer 2 validation).
func HasPermission(permissions []string, required string) bool {
	return slices.Contains(permissions, required)
}

// HasAnyPermission cek apakah punya setidaknya satu dari permissions yang dibutuhkan.
func HasAnyPermission(permissions []string, required ...string) bool {
	for _, r := range required {
		if slices.Contains(permissions, r) {
			return true
		}
	}
	return false
}

// HasAllPermissions cek apakah punya semua permissions yang dibutuhkan.
func HasAllPermissions(permissions []string, required ...string) bool {
	for _, r := range required {
		if !slices.Contains(permissions, r) {
			return false
		}
	}
	return true
}

// HasRole cek apakah slice roles mengandung role yang dibutuhkan.
func HasRole(roles []string, required string) bool {
	return slices.Contains(roles, required)
}

// IsSuperAdmin shortcut — superadmin bypass semua permission check.
func IsSuperAdmin(roles []string) bool {
	return HasRole(roles, "superadmin")
}
