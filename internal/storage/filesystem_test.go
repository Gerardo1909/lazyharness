package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Gerardo1909/lazyharness/internal/domain"
)

func setupTestHarness(t *testing.T) (string, domain.Harness) {
	t.Helper()
	dir := t.TempDir()
	h, _ := domain.NewHarness("test-harness", dir, "xml")
	_ = h.AddRole(domain.Role{Name: "arquitecto", Color: "#f7768e", PromptFile: "arquitecto.xml"})
	return dir, h
}

func TestLoadHarnessShouldReturnHarnessWhenValidFileExists(t *testing.T) {
	// Arrange
	dir, h := setupTestHarness(t)
	if err := SaveHarness(dir, h); err != nil {
		t.Fatalf("error guardando harness: %v", err)
	}
	// Act
	loaded, err := LoadHarness(dir)
	// Assert
	if err != nil {
		t.Fatalf("error cargando harness: %v", err)
	}
	if loaded.Name != h.Name {
		t.Errorf("nombre esperado %q, obtuve %q", h.Name, loaded.Name)
	}
}

func TestLoadHarnessShouldReturnErrorWhenNoFileExists(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	// Act
	_, err := LoadHarness(dir)
	// Assert
	if err == nil {
		t.Error("esperaba error al cargar harness inexistente")
	}
}

func TestSaveHarnessShouldCreateFileAndDirectories(t *testing.T) {
	// Arrange
	dir, h := setupTestHarness(t)
	// Act
	err := SaveHarness(dir, h)
	// Assert
	if err != nil {
		t.Fatalf("error guardando harness: %v", err)
	}
	expectedPath := filepath.Join(dir, HarnessDirName, HarnessFileName)
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Errorf("archivo harness.json no fue creado en %s", expectedPath)
	}
}

func TestSaveHarnessShouldWriteValidJSON(t *testing.T) {
	// Arrange
	dir, h := setupTestHarness(t)
	_ = SaveHarness(dir, h)
	// Act
	data, err := os.ReadFile(filepath.Join(dir, HarnessDirName, HarnessFileName))
	if err != nil {
		t.Fatalf("no se pudo leer el archivo: %v", err)
	}
	var parsed domain.Harness
	// Assert
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Errorf("el JSON guardado no es válido: %v", err)
	}
}

func TestReadWritePromptFileShouldRoundTrip(t *testing.T) {
	// Arrange
	dir, h := setupTestHarness(t)
	_ = SaveHarness(dir, h) // crea el directorio roles/
	content := "<role>\nSos el arquitecto del proyecto.\n</role>"
	// Act
	err := WritePromptFile(dir, "arquitecto.xml", content)
	if err != nil {
		t.Fatalf("error escribiendo prompt: %v", err)
	}
	read, err := ReadPromptFile(dir, "arquitecto.xml")
	// Assert
	if err != nil {
		t.Fatalf("error leyendo prompt: %v", err)
	}
	if read != content {
		t.Errorf("contenido diferente:\nescrito: %q\nleído:   %q", content, read)
	}
}

func TestLoadTasksShouldReturnNilWhenNoFile(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	// Act
	tasks, err := LoadTasks(dir)
	// Assert
	if err != nil {
		t.Errorf("no debería haber error si tareas.json no existe: %v", err)
	}
	if tasks != nil {
		t.Errorf("esperaba nil, obtuve %v", tasks)
	}
}

func TestFindHarnessesShouldReturnSummariesWhenHarnessesExist(t *testing.T) {
	// Arrange
	root := t.TempDir()
	projDir := filepath.Join(root, "mi-proyecto")
	_ = os.Mkdir(projDir, 0755)
	h, _ := domain.NewHarness("mi-harness", projDir, "xml")
	_ = SaveHarness(projDir, h)
	// Act
	summaries := FindHarnesses([]string{root})
	// Assert
	if len(summaries) != 1 {
		t.Errorf("esperaba 1 harness, obtuve %d", len(summaries))
	}
	if summaries[0].Name != "mi-harness" {
		t.Errorf("nombre esperado %q, obtuve %q", "mi-harness", summaries[0].Name)
	}
}
