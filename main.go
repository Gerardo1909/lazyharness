package main

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/Gerardo1909/lazyharness/internal/domain"
)

func main() {
	// Crear un nuevo harness
	harness, error := domain.NewHarness("dev-flow", "/home/user/proj", "xml")
	if error != nil {
		log.Fatalf("Error al crear el harness: %v", error)
	}
	// Agregar roles
	arquitecto, error := domain.NewRole("arquitecto", "#f7768e", "arquitecto.xml", "")
	if error != nil {
		log.Fatalf("Error al crear el rol arquitecto: %v", error)
	}
	codeReviewer, error := domain.NewRole("code-reviewer", "#7aa2f7", "code-reviewer.xml", "arquitecto")
	if error != nil {
		log.Fatalf("Error al crear el rol code-reviewer: %v", error)
	}
	devBackend, error := domain.NewRole("dev-backend", "#9ece6a", "dev-backend.xml", "arquitecto")
	if error != nil {
		log.Fatalf("Error al crear el rol dev-backend: %v", error)
	}
	devFrontend, error := domain.NewRole("dev-frontend", "#e0af68", "dev-frontend.xml", "arquitecto")
	if error != nil {
		log.Fatalf("Error al crear el rol dev-frontend: %v", error)
	}
	docs, error := domain.NewRole("docs", "#bb9af7", "docs.xml", "")
	if error != nil {
		log.Fatalf("Error al crear el rol docs: %v", error)
	}

	roles := []domain.Role{
		arquitecto,
		codeReviewer,
		devBackend,
		devFrontend,
		docs,
	}
	for _, role := range roles {
		error := harness.AddRole(role)
		if error != nil {
			log.Fatalf("Error al agregar el rol %s: %v", role.Name, error)
		}
	}

	harness.Workflow = []string{"arquitecto", "code-reviewer", "dev-backend", "dev-frontend", "docs"}

	// Serializar a JSON
	data, error := json.MarshalIndent(harness, "", "  ")
	if error != nil {
		log.Fatal(error)
	}

	fmt.Println("=== Harness como JSON ===")
	fmt.Println(string(data))

	// Deserializar y verificar
	var loaded domain.Harness
	if err := json.Unmarshal(data, &loaded); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("\n=== Verificacion ===\n")
	fmt.Printf("Nombre: %s\n", loaded.Name)
	fmt.Printf("Roles: %d\n", len(loaded.Roles))
	for _, role := range loaded.Roles {
		fmt.Printf("  %s\n", role.DisplayName())
	}
	fmt.Printf("Workflow: %v\n", loaded.Workflow)
}
