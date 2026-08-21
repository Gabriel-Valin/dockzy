package ui

import (
	"strings"

	"github.com/rivo/tview"
)

type tabbedPanel struct {
	root      *tview.Flex
	header    *tview.TextView
	pages     *tview.Pages
	tabNames  []string
	tabLabels []string
	active    int

	logsView   *tview.TextView
	statsView  *tview.TextView
	configView *tview.TextView
	topView    *tview.TextView
}

func newTabbedPanel(data Data) *tabbedPanel {
	tp := &tabbedPanel{
		tabNames:  []string{"logs", "stats", "config", "top"},
		tabLabels: []string{"Logs", "Stats", "Container Config", "Top"},
		active:    0,
	}

	tp.pages = tview.NewPages()

	logsView := tview.NewTextView().SetDynamicColors(true).SetScrollable(true)
	logsView.SetText(data.Logs)

	statsView := tview.NewTextView().SetDynamicColors(true).SetScrollable(true)
	statsView.SetText(data.Stats)

	configView := tview.NewTextView().SetDynamicColors(true).SetScrollable(true)
	configView.SetText(data.Config)

	topView := tview.NewTextView().SetDynamicColors(true).SetScrollable(true)
	topView.SetText(data.Top)

	tp.pages.AddPage("logs", logsView, true, true)
	tp.pages.AddPage("stats", statsView, true, false)
	tp.pages.AddPage("config", configView, true, false)
	tp.pages.AddPage("top", topView, true, false)

	tp.logsView = logsView
	tp.statsView = statsView
	tp.configView = configView
	tp.topView = topView

	tp.header = tview.NewTextView().SetDynamicColors(true)
	tp.header.SetText(tp.renderHeader())

	tp.root = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(tp.header, 1, 0, false).
		AddItem(tp.pages, 0, 1, true)

	tp.root.SetBorder(true).SetTitleAlign(tview.AlignLeft)

	return tp
}

func (tp *tabbedPanel) renderHeader() string {
	parts := make([]string, len(tp.tabLabels))
	for i, label := range tp.tabLabels {
		if i == tp.active {
			parts[i] = "[yellow]" + label + "[white]"
		} else {
			parts[i] = "[gray]" + label + "[white]"
		}
	}
	return strings.Join(parts, " - ")
}

func (tp *tabbedPanel) next() {
	tp.active = (tp.active + 1) % len(tp.tabNames)
	tp.sync()
}

func (tp *tabbedPanel) prev() {
	tp.active = (tp.active - 1 + len(tp.tabNames)) % len(tp.tabNames)
	tp.sync()
}

func (tp *tabbedPanel) sync() {
	tp.pages.SwitchToPage(tp.tabNames[tp.active])
	tp.header.SetText(tp.renderHeader())
}

func (tp *tabbedPanel) setContent(logs, stats, config, top string) {
	tp.logsView.SetText(logs)
	tp.statsView.SetText(stats)
	tp.configView.SetText(config)
	tp.topView.SetText(top)
}

func (tp *tabbedPanel) setStats(stats string) {
	tp.statsView.SetText(stats)
}
