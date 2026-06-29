package tui

import "github.com/Gerardo1909/lazyharness/internal/domain"

// Mensajes de navegación entre pantallas. Los emiten las sub-pantallas
// y los captura el modelo raíz (App) para hacer el routing.

type OpenHarnessMsg struct{ Summary domain.HarnessSummary }
type OpenEditorMsg struct {
	Harness  domain.Harness
	RoleName string
}
type OpenHistoryMsg struct {
	Harness  domain.Harness
	RoleName string
}
type OpenWorkspaceMsg struct{ Harness domain.Harness }
type GoBackMsg struct{}
type SaveAndBackMsg struct{ CommitMessage string }
type SplashDoneMsg struct{}
