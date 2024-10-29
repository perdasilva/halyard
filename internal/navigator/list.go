package navigator

import (
	"fmt"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

var _ tea.Model = listModel{}

type item struct {
	title       string
	description string
}

func (i item) Title() string       { return i.title }
func (i item) Description() string { return i.description }
func (i item) FilterValue() string { return i.title }

type ItemSelectedMsg struct {
	Item item
}

type listModel struct {
	list.Model
	choose key.Binding
}

func NewList() tea.Model {
	m := listModel{}
	itemDelegate := list.NewDefaultDelegate()
	itemDelegate.ShortHelpFunc = m.ShortHelp
	itemDelegate.FullHelpFunc = m.FullHelp
	itemDelegate.UpdateFunc = m.listUpdate
	itemDelegate.SetSpacing(0)
	itemDelegate.ShowDescription = false

	items := make([]list.Item, 0, 25)
	for i := 0; i < 25; i++ {
		items = append(items, item{
			title:       fmt.Sprintf("Item %d", i),
			description: fmt.Sprintf("Item %d", i),
		})
	}
	m.Model = list.New(items, itemDelegate, 0, 0)
	return m
}

func (m listModel) Init() tea.Cmd {
	return nil
}

func (m listModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		h, v := appStyle.GetFrameSize()
		m.SetSize(msg.Width-h, msg.Height-v)
	}
	newListModel, cmd := m.Model.Update(msg)
	m.Model = newListModel
	return m, cmd
}

func (m listModel) View() string {
	return appStyle.Render(m.Model.View())
}

func (m listModel) ShortHelp() []key.Binding {
	return []key.Binding{m.choose}
}

func (m listModel) FullHelp() [][]key.Binding {
	return [][]key.Binding{{m.choose}}
}

func (m listModel) listUpdate(msg tea.Msg, _ *list.Model) tea.Cmd {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.choose):
			if i, ok := m.SelectedItem().(item); ok {
				return func() tea.Msg {
					return ItemSelectedMsg{Item: i}
				}
			}
		}
	}
	return nil
}
