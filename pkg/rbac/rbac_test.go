package rbac

import "testing"

func TestHasPermission(t *testing.T) {
	tests := []struct {
		name        string
		permissions []string
		required    string
		want        bool
	}{
		{"has permission", []string{"packages.create", "packages.read"}, "packages.create", true},
		{"missing permission", []string{"packages.read"}, "packages.create", false},
		{"empty permissions", []string{}, "anything", false},
		{"nil permissions", nil, "anything", false},
		{"exact match", []string{"admin"}, "admin", true},
		{"case sensitive", []string{"Admin"}, "admin", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HasPermission(tt.permissions, tt.required)
			if got != tt.want {
				t.Errorf("HasPermission(%v, %q) = %v, want %v", tt.permissions, tt.required, got, tt.want)
			}
		})
	}
}

func TestHasAnyPermission(t *testing.T) {
	tests := []struct {
		name        string
		permissions []string
		required    []string
		want        bool
	}{
		{"has one of many", []string{"packages.read"}, []string{"packages.create", "packages.read", "packages.delete"}, true},
		{"has none", []string{"packages.read"}, []string{"packages.create", "packages.delete"}, false},
		{"empty required", []string{"packages.read"}, []string{}, false},
		{"nil permissions", nil, []string{"anything"}, false},
		{"multiple match", []string{"a", "b", "c"}, []string{"b", "d"}, true},
		{"exact match all", []string{"admin"}, []string{"admin"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HasAnyPermission(tt.permissions, tt.required...)
			if got != tt.want {
				t.Errorf("HasAnyPermission(%v, %v) = %v, want %v", tt.permissions, tt.required, got, tt.want)
			}
		})
	}
}

func TestHasAllPermissions(t *testing.T) {
	tests := []struct {
		name        string
		permissions []string
		required    []string
		want        bool
	}{
		{"has all", []string{"a", "b", "c"}, []string{"a", "b"}, true},
		{"missing one", []string{"a", "b"}, []string{"a", "b", "c"}, false},
		{"empty required", []string{"a"}, []string{}, true},
		{"nil permissions", nil, []string{"a"}, false},
		{"same order", []string{"a", "b"}, []string{"a", "b"}, true},
		{"different order", []string{"b", "a"}, []string{"a", "b"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HasAllPermissions(tt.permissions, tt.required...)
			if got != tt.want {
				t.Errorf("HasAllPermissions(%v, %v) = %v, want %v", tt.permissions, tt.required, got, tt.want)
			}
		})
	}
}

func TestHasRole(t *testing.T) {
	tests := []struct {
		name     string
		roles    []string
		required string
		want     bool
	}{
		{"has role", []string{"admin", "manager"}, "admin", true},
		{"missing role", []string{"viewer"}, "admin", false},
		{"empty roles", []string{}, "admin", false},
		{"nil roles", nil, "admin", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HasRole(tt.roles, tt.required)
			if got != tt.want {
				t.Errorf("HasRole(%v, %q) = %v, want %v", tt.roles, tt.required, got, tt.want)
			}
		})
	}
}

func TestIsSuperAdmin(t *testing.T) {
	tests := []struct {
		name  string
		roles []string
		want  bool
	}{
		{"is superadmin", []string{"superadmin"}, true},
		{"superadmin with other roles", []string{"admin", "superadmin"}, true},
		{"not superadmin", []string{"admin"}, false},
		{"empty roles", []string{}, false},
		{"nil roles", nil, false},
		{"case sensitive", []string{"Superadmin"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsSuperAdmin(tt.roles)
			if got != tt.want {
				t.Errorf("IsSuperAdmin(%v) = %v, want %v", tt.roles, got, tt.want)
			}
		})
	}
}
