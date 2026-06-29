package tui

import "github.com/charmbracelet/lipgloss"

// En el futuro si se quieren implementar temas,
// se arma un struct que se pase como parametro
var (
	ColorBg             = lipgloss.Color("#16161e")
	ColorPanelBg        = lipgloss.Color("#1a1b26")
	ColorFg             = lipgloss.Color("#c0caf5")
	ColorBlue           = lipgloss.Color("#7aa2f7")
	ColorGreen          = lipgloss.Color("#9ece6a")
	ColorRed            = lipgloss.Color("#f7768e")
	ColorYellow         = lipgloss.Color("#e0af68")
	ColorPurple         = lipgloss.Color("#bb9af7")
	ColorCyan           = lipgloss.Color("#7dcfff")
	ColorComment        = lipgloss.Color("#565f89")
	ColorSelection      = lipgloss.Color("#283457")
	ColorBorderInactive = lipgloss.Color("#3b4261")
)

// Estilos base reutilizables
var (
	StyleTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorBlue)

	StyleSubtitle = lipgloss.NewStyle().
			Foreground(ColorComment)

	StyleHighlight = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorFg).
			Background(ColorSelection)

	StyleHelp = lipgloss.NewStyle().
			Foreground(ColorComment)

	StyleBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorderInactive)

	StyleActiveBorder = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorBlue)

	// StyleKeybar es la barra de atajos inferior, reutilizable en todas las pantallas.
	StyleKeybar = lipgloss.NewStyle().Background(ColorPanelBg).Padding(0, 1)
	// StyleKeyHi resalta la tecla en la keybar.
	StyleKeyHi = lipgloss.NewStyle().Foreground(ColorBlue)
)
