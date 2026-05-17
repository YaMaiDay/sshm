package tui

import "github.com/charmbracelet/lipgloss"

var (
	green     = lipgloss.Color("42")
	yellow    = lipgloss.Color("214")
	red       = lipgloss.Color("196")
	blue      = lipgloss.Color("39")
	textGray  = lipgloss.Color("245")
	valueGray = lipgloss.Color("252")
	cyan      = lipgloss.Color("45")
	softGray  = lipgloss.Color("235")
	lineGray  = lipgloss.Color("234")

	titleStyle          = lipgloss.NewStyle().Bold(true).Foreground(blue)
	mutedStyle          = lipgloss.NewStyle().Foreground(textGray)
	cardMutedStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("248"))
	helpStyle           = lipgloss.NewStyle().Foreground(textGray)
	navStyle            = lipgloss.NewStyle().Foreground(textGray)
	barEmptyStyle       = lipgloss.NewStyle().Foreground(softGray)
	subtleLineStyle     = lipgloss.NewStyle().Foreground(lineGray)
	detailSectionStyle  = lipgloss.NewStyle().Bold(true).Foreground(blue)
	detailSubTitleStyle = lipgloss.NewStyle().Foreground(cyan)
	detailSuccessStyle  = lipgloss.NewStyle().Foreground(green)
	detailDangerStyle   = lipgloss.NewStyle().Foreground(red)
	detailLabelStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	detailValueStyle    = lipgloss.NewStyle().Foreground(valueGray)
	detailSizeStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("15"))

	cardStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(softGray).
			Padding(0, 1).
			MarginBottom(0)
	selectedCardStyle       = cardStyle.BorderForeground(blue)
	cardBorderStyle         = lipgloss.NewStyle().Foreground(softGray)
	selectedCardBorderStyle = lipgloss.NewStyle().Foreground(blue)
	detailStyle             = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(blue).
				Padding(1, 2)

	greenStyle    = lipgloss.NewStyle().Foreground(green)
	yellowStyle   = lipgloss.NewStyle().Foreground(yellow)
	favoriteStyle = lipgloss.NewStyle().Bold(true).Foreground(yellow)
	pinnedStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("201"))
	redStyle      = lipgloss.NewStyle().Foreground(red)
	blueStyle     = lipgloss.NewStyle().Foreground(blue)
)
