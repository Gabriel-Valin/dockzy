package ui

import "github.com/rivo/tview"

const (
	detailPageContainer = "container"
	detailPageResource  = "resource"
)

type detailPanel struct {
	root       *tview.Pages
	tabs       *tabbedPanel
	configView *tview.TextView

	resourceActive bool
}

func newDetailPanel(data Data) *detailPanel {
	tabs := newTabbedPanel(data)

	configView := tview.NewTextView().SetDynamicColors(true).SetScrollable(true)
	configView.SetBorder(true).SetTitle(" Config ").SetTitleAlign(tview.AlignLeft)

	pages := tview.NewPages().
		AddPage(detailPageContainer, tabs.root, true, true).
		AddPage(detailPageResource, configView, true, false)

	return &detailPanel{root: pages, tabs: tabs, configView: configView}
}

func (dp *detailPanel) showContainer(logs, stats, config, top string) {
	dp.tabs.setContent(logs, stats, config, top)
	dp.resourceActive = false
	dp.root.SwitchToPage(detailPageContainer)
}

func (dp *detailPanel) updateStats(text string) {
	dp.tabs.setStats(text)
}

func (dp *detailPanel) showResource(text string) {
	dp.configView.SetText(text)
	dp.resourceActive = true
	dp.root.SwitchToPage(detailPageResource)
}

func (dp *detailPanel) prevTab() {
	if dp.resourceActive {
		return
	}
	dp.tabs.prev()
}

func (dp *detailPanel) nextTab() {
	if dp.resourceActive {
		return
	}
	dp.tabs.next()
}
