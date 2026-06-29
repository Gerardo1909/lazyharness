package app

import (
	"github.com/Gerardo1909/lazyharness/internal/domain"
	"github.com/Gerardo1909/lazyharness/internal/storage"
	"github.com/Gerardo1909/lazyharness/internal/tui"
	"github.com/Gerardo1909/lazyharness/internal/tui/editor"
	"github.com/Gerardo1909/lazyharness/internal/tui/harness"
	"github.com/Gerardo1909/lazyharness/internal/tui/history"
	"github.com/Gerardo1909/lazyharness/internal/tui/home"
	"github.com/Gerardo1909/lazyharness/internal/tui/splash"
	"github.com/Gerardo1909/lazyharness/internal/tui/workspace"
	tea "github.com/charmbracelet/bubbletea"
)

type screen int

const (
	screenSplash screen = iota
	screenHome
	screenHarness
	screenEditor
	screenHistory
	screenWorkspace
)

type App struct {
	screen      screen
	splash      splash.Model
	home        home.Model
	harnessV    harness.Model
	editorV     editor.Model
	historyV    history.Model
	workspaceV  workspace.Model
	width       int
	height      int
}

func NewApp() App {
	return App{screen: screenSplash}
}

func (a App) Init() tea.Cmd { return nil }

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width, a.height = msg.Width, msg.Height
		if a.screen == screenSplash {
			a.splash = splash.New(msg.Width, msg.Height)
			return a, nil
		}

	case tui.SplashDoneMsg:
		a.home = home.New(a.width, a.height)
		a.screen = screenHome
		return a, a.home.Init()

	case tui.OpenHarnessMsg:
		h := loadHarness(msg.Summary)
		a.harnessV = harness.New(h, a.width, a.height)
		a.screen = screenHarness
		return a, a.harnessV.Init()

	case tui.OpenEditorMsg:
		a.editorV = editor.New(msg.Harness, msg.RoleName, a.width, a.height)
		a.screen = screenEditor
		return a, a.editorV.Init()

	case tui.OpenHistoryMsg:
		a.historyV = history.New(msg.Harness, msg.RoleName, a.width, a.height)
		a.screen = screenHistory
		return a, a.historyV.Init()

	case tui.OpenWorkspaceMsg:
		a.workspaceV = workspace.New(msg.Harness, a.width, a.height)
		a.screen = screenWorkspace
		return a, a.workspaceV.Init()

	case tui.GoBackMsg, tui.SaveAndBackMsg:
		switch a.screen {
		case screenHarness:
			a.home = home.New(a.width, a.height)
			a.screen = screenHome
		case screenEditor, screenHistory:
			h := loadHarness(a.harnessV.HarnessSummary())
			a.harnessV = harness.New(h, a.width, a.height)
			a.screen = screenHarness
		case screenWorkspace:
			a.screen = screenHarness
		default:
			a.screen = screenHome
		}
		return a, nil
	}

	switch a.screen {
	case screenSplash:
		var cmd tea.Cmd
		a.splash, cmd = a.splash.Update(msg)
		return a, cmd
	case screenHome:
		var cmd tea.Cmd
		a.home, cmd = a.home.Update(msg)
		return a, cmd
	case screenHarness:
		var cmd tea.Cmd
		a.harnessV, cmd = a.harnessV.Update(msg)
		return a, cmd
	case screenEditor:
		var cmd tea.Cmd
		a.editorV, cmd = a.editorV.Update(msg)
		return a, cmd
	case screenHistory:
		var cmd tea.Cmd
		a.historyV, cmd = a.historyV.Update(msg)
		return a, cmd
	case screenWorkspace:
		var cmd tea.Cmd
		a.workspaceV, cmd = a.workspaceV.Update(msg)
		return a, cmd
	}
	return a, nil
}

func (a App) View() string {
	switch a.screen {
	case screenHome:
		return a.home.View()
	case screenHarness:
		return a.harnessV.View()
	case screenEditor:
		return a.editorV.View()
	case screenHistory:
		return a.historyV.View()
	case screenWorkspace:
		return a.workspaceV.View()
	default:
		return a.splash.View()
	}
}

func loadHarness(s domain.HarnessSummary) domain.Harness {
	h, err := storage.LoadHarness(s.ProjectDir)
	if err == nil {
		return h
	}
	return domain.Harness{
		Name:         s.Name,
		ProjectDir:   s.ProjectDir,
		PromptFormat: s.PromptFormat,
		Provider:     s.Provider,
		Model:        s.Model,
		Roles:        s.Roles,
		Workflow:     s.Workflow,
	}
}
