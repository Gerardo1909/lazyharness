package domain

import (
	"fmt"
	"time"
)

// representa un rol dentro del harness
type Role struct {
	Name       string `json:"name"`
	Color      string `json:"color"`
	PromptFile string `json:"prompt_file"`
	Parent     string `json:"parent,omitempty"`
}

// color tomado por defecto por un rol
const DefaultColor = "#c0caf5"

func (r Role) DisplayName() string {
	var color string = r.Color
	if color == "" {
		color = DefaultColor
	}
	return fmt.Sprintf("[%s] %s", color, r.Name)
}

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

// Funcion para generar nuevos harnesses
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

// Aca se apunta al harness porque modificamos uno de sus campos
func (h *Harness) AddRole(role Role) error {
	for _, existingRole := range h.Roles {
		if existingRole.Name == role.Name {
			return fmt.Errorf("El rol %s ya existe en el harness", role.Name)
		}
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
