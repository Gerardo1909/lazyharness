package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Gerardo1909/lazyharness/internal/domain"
)

const (
	HarnessDirName  = ".lazyharness"
	HarnessFileName = "harness.json"
	RolesDirName    = "roles"
	TasksFileName   = "tareas.json"
)

// FindHarnesses recorre searchRoots buscando proyectos con .lazyharness/ y devuelve sus resúmenes.
func FindHarnesses(searchRoots []string) []domain.HarnessSummary {
	var summaries []domain.HarnessSummary
	for _, root := range searchRoots {
		root = expandHome(root)
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			projectPath := filepath.Join(root, entry.Name())
			harnessPath := filepath.Join(projectPath, HarnessDirName, HarnessFileName)
			if _, err := os.Stat(harnessPath); os.IsNotExist(err) {
				continue
			}
			h, err := LoadHarness(projectPath)
			if err != nil {
				continue
			}
			tasks, _ := LoadTasks(projectPath)
			summaries = append(summaries, h.Summary(tasks))
		}
	}
	return summaries
}

// LoadHarness lee harness.json desde projectPath/.lazyharness/
func LoadHarness(projectPath string) (domain.Harness, error) {
	path := filepath.Join(projectPath, HarnessDirName, HarnessFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		return domain.Harness{}, fmt.Errorf("leyendo harness.json: %w", err)
	}
	var h domain.Harness
	if err := json.Unmarshal(data, &h); err != nil {
		return domain.Harness{}, fmt.Errorf("parseando harness.json: %w", err)
	}
	h.ProjectDir = projectPath
	return h, nil
}

// SaveHarness escribe harness.json en projectPath/.lazyharness/
func SaveHarness(projectPath string, h domain.Harness) error {
	dir := filepath.Join(projectPath, HarnessDirName)
	if err := os.MkdirAll(filepath.Join(dir, RolesDirName), 0755); err != nil {
		return fmt.Errorf("creando directorios: %w", err)
	}
	data, err := json.MarshalIndent(h, "", "  ")
	if err != nil {
		return fmt.Errorf("serializando harness: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, HarnessFileName), data, 0644); err != nil {
		return err
	}
	_ = InitRepo(projectPath)
	return nil
}

// SaveTasks escribe tareas.json en el directorio .lazyharness/
func SaveTasks(projectPath string, tasks []domain.Task) error {
	path := filepath.Join(projectPath, HarnessDirName, TasksFileName)
	data, err := json.MarshalIndent(tasks, "", "  ")
	if err != nil {
		return fmt.Errorf("serializando tareas: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

// ReadPromptFile lee el contenido de un archivo de prompt de un rol.
func ReadPromptFile(projectPath, promptFile string) (string, error) {
	path := filepath.Join(projectPath, HarnessDirName, RolesDirName, promptFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("leyendo prompt %s: %w", promptFile, err)
	}
	return string(data), nil
}

// WritePromptFile escribe el contenido de un prompt de forma atómica (temp + rename).
func WritePromptFile(projectPath, promptFile, content string) error {
	path := filepath.Join(projectPath, HarnessDirName, RolesDirName, promptFile)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(content), 0644); err != nil {
		return fmt.Errorf("escribiendo prompt temp: %w", err)
	}
	return os.Rename(tmp, path)
}

// LoadTasks lee tareas.json del harness. Devuelve nil si el archivo no existe.
func LoadTasks(projectPath string) ([]domain.Task, error) {
	path := filepath.Join(projectPath, HarnessDirName, TasksFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("leyendo tareas.json: %w", err)
	}
	var tasks []domain.Task
	return tasks, json.Unmarshal(data, &tasks)
}

// HarnessPath devuelve la ruta completa del directorio .lazyharness/
func HarnessPath(projectPath string) string {
	return filepath.Join(projectPath, HarnessDirName)
}

func expandHome(path string) string {
	if len(path) > 0 && path[0] == '~' {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[1:])
	}
	return path
}
