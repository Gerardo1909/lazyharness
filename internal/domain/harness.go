package domain

import (
	"fmt"
	"time"
)

// LOGICA DE ROLES
type Role struct {
	Name       string `json:"name"`
	Color      string `json:"color"`
	PromptFile string `json:"prompt_file"`
	Parent     string `json:"parent,omitempty"`
}

// color tomado por defecto por un rol
const DefaultColor = "#c0caf5"

func validateHexColor(color string) bool {
	if len(color) != 7 || color[0] != '#' {
		return false
	}
	for _, c := range color[1:] {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
			return false
		}
	}
	return true
}

func getFileExtension(filename string) string {
	for i := len(filename) - 1; i >= 0; i-- {
		if filename[i] == '.' {
			return filename[i+1:]
		}
	}
	return ""
}

func (r Role) validateRole() error {
	if r.Name == "" {
		return fmt.Errorf("El nombre del rol no puede estar vacio")
	}
	validPromptFormats := map[string]bool{
		"xml": true,
		"md":  true,
		"txt": true,
	}
	// Validar que el formato del prompt sea válido
	promptFormat := getFileExtension(r.PromptFile)
	if !validPromptFormats[promptFormat] {
		return fmt.Errorf("El formato del prompt no es valido. Formatos validos: xml, md, txt")
	}
	// Validar que el color sea un código hexadecimal válido
	if !validateHexColor(r.Color) {
		return fmt.Errorf("El color no es un código hexadecimal válido")
	}
	return nil
}

func (r Role) DisplayName() string {
	var color string = r.Color
	if color == "" {
		color = DefaultColor
	}
	return fmt.Sprintf("[%s] %s", color, r.Name)
}

func NewRole(name string, color string, promptFile string, parent string) (Role, error) {
	role := Role{
		Name:       name,
		Color:      color,
		PromptFile: promptFile,
		Parent:     parent,
	}
	error := role.validateRole()
	if error != nil {
		return Role{}, error
	}
	return role, nil
}

// LOGICA DE HARNESS
// Harness es basicamente un conjunto de roles + metadata para diferenciarlo
type Harness struct {
	Name         string   `json:"name"`
	ProjectDir   string   `json:"project_dir"`
	PromptFormat string   `json:"prompt_format"`
	Provider     string   `json:"provider,omitempty"`
	Model        string   `json:"model,omitempty"`
	Roles        []Role   `json:"roles"`
	Workflow     []string `json:"workflow"`
	CreatedAt    string   `json:"created_at"`
}

// Resumen del harness que se muestra en la primera vista del CLI
type HarnessSummary struct {
	Name       string `json:"name"`
	ProjectDir string `json:"project_dir"`
	RoleCount  int8   `json:"role_count"`
	LastCommit string `json:"last_commit"`
	Provider   string `json:"provider,omitempty"`
}

func NewHarness(name string, projectDir string, promptFormat string) (Harness, error) {
	if name == "" {
		return Harness{}, fmt.Errorf("El nombre del harness no puede estart vacio")
	}
	if projectDir == "" {
		return Harness{}, fmt.Errorf("La ruta del directorio no puede estar vacia")
	}
	validPromptFormats := map[string]bool{
		"xml": true,
		"md":  true,
		"txt": true,
	}
	if !validPromptFormats[promptFormat] {
		return Harness{}, fmt.Errorf("El formato del prompt no es valido. Formatos validos: xml, md, txt")
	}
	return Harness{
		Name:         name,
		ProjectDir:   projectDir,
		PromptFormat: promptFormat,
		Roles:        []Role{},
		Workflow:     []string{},
		CreatedAt:    time.Now().Format(time.RFC3339),
	}, nil
}

func (h *Harness) AddRole(role Role) error {
	for _, existingRole := range h.Roles {
		if existingRole.Name == role.Name {
			return fmt.Errorf("El rol %s ya existe en el harness", role.Name)
		}
	}
	if h.PromptFormat != getFileExtension(role.PromptFile) {
		return fmt.Errorf("El formato del prompt del rol no coincide con el formato definido en el harness")
	}
	h.Roles = append(h.Roles, role)
	return nil
}

// Aca tomamos el harness completo porque lo leemos unicamente
func (h Harness) FindRoleByName(name string) (Role, bool) {
	for _, role := range h.Roles {
		if role.Name == name {
			return role, true
		}
	}
	return Role{}, false
}

func (h Harness) RoleNames() []string {
	names := make([]string, len(h.Roles))
	for i, role := range h.Roles {
		names[i] = role.Name
	}
	return names
}
