package domain

import (
	"testing"
)

func TestHarnessShouldBeCreatedWhenValidParamsSubmitted(t *testing.T) {
	// Arrange
	tests := []struct {
		name        string
		harnessName string
		format      string
		dir         string
		wantErr     bool
	}{
		{"valido", "dev-flow", "xml", "/home/user/proj", false},
		{"nombre vacio", "", "xml", "/home/user/proj", true},
		{"formato invalido", "dev-flow", "yaml", "/home/user/proj", true},
		{"dir vacio", "dev-flow", "xml", "", true},
		{"formato md", "dev-flow", "md", "/home/user/proj", false},
		{"formato txt", "dev-flow", "txt", "/home/user/proj", false},
	}
	// Act
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewHarness(tt.harnessName, tt.dir, tt.format)

			// Assert
			if (err != nil) != tt.wantErr {
				t.Errorf("NewHarness() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNameShouldBeDisplayedWhenValidRoleCreated(t *testing.T) {
	// Arrange
	tests := []struct {
		name     string
		role     Role
		expected string
	}{
		{
			name:     "rol con color",
			role:     Role{Name: "arquitecto", Color: "#f7768e"},
			expected: "[#f7768e] arquitecto",
		},
		{
			name:     "rol sin color usa default",
			role:     Role{Name: "docs", Color: ""},
			expected: "[#c0caf5] docs",
		},
	}
	// Act
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.role.DisplayName()
			// Assert
			if got != tt.expected {
				t.Errorf("esperaba %q, obtuve %q", tt.expected, got)
			}
		})
	}
}

func TestRoleShouldBeAddedToHarnessWhenValidParamsProvided(t *testing.T) {
	// Arrange
	harness, _ := NewHarness("dev-flow", "xml", "/home/user/proj")
	role := Role{Name: "arquitecto", Color: "#f7768e"}
	// Act
	harness.AddRole(role)
	// Assert
	if len(harness.Roles) != 1 {
		t.Errorf("esperaba 1 rol, obtuve %d", len(harness.Roles))
	}
	if harness.Roles[0].Name != role.Name {
		t.Errorf("esperaba rol con nombre %q, obtuve %q", role.Name, harness.Roles[0].Name)
	}
	if harness.Roles[0].Color != role.Color {
		t.Errorf("esperaba rol con color %q, obtuve %q", role.Color, harness.Roles[0].Color)
	}
}

func TestRoleShouldNotBeAddedWhenDuplicateRoleNameProvided(t *testing.T) {
	// Arrange
	harness, _ := NewHarness("dev-flow", "xml", "/home/user/proj")
	role1 := Role{Name: "arquitecto", Color: "#f7768e"}
	role2 := Role{Name: "arquitecto", Color: "#f7768e"}
	// Act
	harness.AddRole(role1)
	error := harness.AddRole(role2)
	// Assert
	if error == nil {
		t.Errorf("esperaba error por rol duplicado, pero no se produjo")
	}
}

func TestFindRoleShouldReturnRoleWhenRoleExists(t *testing.T) {
	// Arrange
	harness, _ := NewHarness("dev-flow", "xml", "/home/user/proj")
	role := Role{Name: "arquitecto", Color: "#f7768e"}
	harness.AddRole(role)
	// Act
	foundRole, err := harness.FindRoleByName("arquitecto")
	// Assert
	if err == false {
		t.Errorf("se deberia encontrar, pero no fue asi: %v", err)
	}
	if foundRole.Name != role.Name {
		t.Errorf("esperaba rol con nombre %q, obtuve %q", role.Name, foundRole.Name)
	}
	if foundRole.Color != role.Color {
		t.Errorf("esperaba rol con color %q, obtuve %q", role.Color, foundRole.Color)
	}
}

func TestFindRoleShouldReturnFalseWhenRoleDoesNotExist(t *testing.T) {
	// Arrange
	harness, _ := NewHarness("dev-flow", "xml", "/home/user/proj")
	// Act
	_, found := harness.FindRoleByName("no-existe")
	// Assert
	if found {
		t.Errorf("esperaba false por rol no existente, pero no lo obtuve")
	}
}
