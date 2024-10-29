package navigator

import (
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	boxer "github.com/treilik/bubbleboxer"
)

var (
	appStyle = lipgloss.NewStyle().Padding(1, 2)

	titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFDF5")).
			Background(lipgloss.Color("#25A065")).
			Padding(0, 1)

	statusMessageStyle = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "#04B575", Dark: "#04B575"}).
				Render
)

type model struct {
	layout         boxer.Boxer
	focusedListIdx int
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	selectedIdx := fmt.Sprintf("%d", m.focusedListIdx)
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "tab":
			m.focusedListIdx = (m.focusedListIdx + 1) % len(m.layout.ModelMap)
			return m, nil
		}
	case tea.WindowSizeMsg:
		_ = m.layout.UpdateSize(msg)
	}

	selectedList := m.layout.ModelMap[selectedIdx]
	newListModel, cmd := selectedList.Update(msg)
	m.layout.ModelMap[selectedIdx] = newListModel
	cmds = append(cmds, cmd)
	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	r := m.layout.View()
	return r
	// return m.lists[0].View()
}

func New() tea.Model {
	layout := boxer.Boxer{}

	leftList, _ := layout.CreateLeaf("0", NewList())
	middleList, _ := layout.CreateLeaf("1", NewList())
	rightList, _ := layout.CreateLeaf("2", NewList())

	layout.LayoutTree = boxer.Node{
		VerticalStacked: false,
		SizeFunc: func(node boxer.Node, widthOrHeight int) []int {
			fraction := widthOrHeight / len(layout.ModelMap)
			return []int{
				widthOrHeight - (len(layout.ModelMap)-1)*fraction,
				fraction,
				fraction,
			}
		},
		Children: []boxer.Node{leftList, middleList, rightList},
	}
	return &model{
		layout: layout,
	}
}
