package tui

import "github.com/charmbracelet/bubbles/key"

// HomeKeys keybindings de la pantalla home.
type HomeKeys struct {
	Up     key.Binding
	Down   key.Binding
	Enter  key.Binding
	New    key.Binding
	Delete key.Binding
	Help   key.Binding
	Quit   key.Binding
}

var HomeKeyMap = HomeKeys{
	Up:     key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "subir")),
	Down:   key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "bajar")),
	Enter:  key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "abrir")),
	New:    key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "nuevo harness")),
	Delete: key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "eliminar")),
	Help:   key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "ayuda")),
	Quit:   key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "salir")),
}

// HarnessKeys keybindings de la vista de harness.
type HarnessKeys struct {
	Up      key.Binding
	Down    key.Binding
	Tab     key.Binding
	Edit    key.Binding
	Delete  key.Binding
	Improve key.Binding
	History key.Binding
	Invoke  key.Binding
	Save    key.Binding
	Tasks   key.Binding
	Back    key.Binding
	Quit    key.Binding
}

var HarnessKeyMap = HarnessKeys{
	Up:      key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "roles")),
	Down:    key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "roles")),
	Tab:     key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "cambiar panel")),
	Edit:    key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "editar")),
	Delete:  key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "eliminar")),
	Improve: key.NewBinding(key.WithKeys("m"), key.WithHelp("m", "mejorar")),
	History: key.NewBinding(key.WithKeys("h"), key.WithHelp("h", "historial")),
	Invoke:  key.NewBinding(key.WithKeys("i"), key.WithHelp("i", "invocar")),
	Save:    key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "guardar (commit)")),
	Tasks:   key.NewBinding(key.WithKeys("t"), key.WithHelp("t", "tareas")),
	Back:    key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "volver")),
	Quit:    key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "salir")),
}

// EditorKeys keybindings del editor de prompts.
type EditorKeys struct {
	Save   key.Binding
	AI     key.Binding
	Discard key.Binding
}

var EditorKeyMap = EditorKeys{
	Save:    key.NewBinding(key.WithKeys("ctrl+s"), key.WithHelp("ctrl+s", "guardar (commit)")),
	AI:      key.NewBinding(key.WithKeys("f2"), key.WithHelp("F2", "activar/ocultar IA")),
	Discard: key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "salir sin guardar")),
}

// HistoryKeys keybindings del historial.
type HistoryKeys struct {
	Up      key.Binding
	Down    key.Binding
	Tab     key.Binding
	Restore key.Binding
	All     key.Binding
	Back    key.Binding
	Quit    key.Binding
}

var HistoryKeyMap = HistoryKeys{
	Up:      key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "versiones")),
	Down:    key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "versiones")),
	Tab:     key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "panel")),
	Restore: key.NewBinding(key.WithKeys("R"), key.WithHelp("R", "restaurar esta versión")),
	All:     key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "historial completo")),
	Back:    key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "volver al harness")),
	Quit:    key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "salir")),
}
