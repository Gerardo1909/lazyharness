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
	roles := []domain.Role{
		{Name: "arquitecto", Color: "#f7768e", PromptFile: "arquitecto.xml"},
		{Name: "code-reviewer", Color: "#7aa2f7", PromptFile: "code-reviewer.xml", Parent: "arquitecto"},
		{Name: "dev-backend", Color: "#9ece6a", PromptFile: "dev-backend.xml", Parent: "arquitecto"},
		{Name: "dev-frontend", Color: "#e0af68", PromptFile: "dev-frontend.xml", Parent: "arquitecto"},
		{Name: "docs", Color: "#bb9af7", PromptFile: "docs.xml"},
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
