package ui

import (
	"strings"

	"github.com/rivo/tview"
)

// ============================================================================
// PAINEL DIREITO (abas)
//
// Uso tview.Pages pra empilhar as 4 abas + um TextView de cabeçalho que
// mostra "Logs - Stats - Container Config - Top" com a ativa destacada.
// ============================================================================

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

	// Uma página (TextView com scroll) por aba.
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

	// Cabeçalho de abas.
	tp.header = tview.NewTextView().SetDynamicColors(true)
	tp.header.SetText(tp.renderHeader())

	// Empilha cabeçalho (1 linha) + páginas.
	tp.root = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(tp.header, 1, 0, false).
		AddItem(tp.pages, 0, 1, true)

	tp.root.SetBorder(true).SetTitleAlign(tview.AlignLeft)

	return tp
}

// renderHeader monta a linha "Logs - Stats - ..." com a aba ativa em amarelo.
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

// next/prev trocam a aba ativa e atualizam cabeçalho + página visível.
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

// setContent troca o texto das quatro abas, ex. ao selecionar um novo
// container. Não mexe na aba ativa nem na posição de scroll das outras.
func (tp *tabbedPanel) setContent(logs, stats, config, top string) {
	tp.logsView.SetText(logs)
	tp.statsView.SetText(stats)
	tp.configView.SetText(config)
	tp.topView.SetText(top)
}

// setStats troca só o texto da aba Stats. Usado a cada amostra do stream de
// stats do container selecionado (~1x por segundo) — não mexe em
// Logs/Config/Top, que não mudam nesse ritmo.
func (tp *tabbedPanel) setStats(stats string) {
	tp.statsView.SetText(stats)
}
