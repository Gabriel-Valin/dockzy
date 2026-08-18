package ui

import "github.com/rivo/tview"

// ============================================================================
// PAINEL DIREITO — troca de "modo" conforme o tipo de seleção
//
// Container (Service/Standalone) usa o painel de 4 abas (tabbedPanel):
// Logs, Stats, Container Config, Top. Image/Volume não têm logs, stats ao
// vivo nem processos "top" — só faz sentido mostrar a config (inspect), daí
// um segundo painel, mais simples, com um único bloco de texto. Os dois
// convivem numa tview.Pages; só um fica visível por vez.
// ============================================================================

const (
	detailPageContainer = "container"
	detailPageResource  = "resource"
)

type detailPanel struct {
	root       *tview.Pages
	tabs       *tabbedPanel
	configView *tview.TextView

	// resourceActive indica se a página visível agora é a de Config
	// (imagem/volume) — usado pra Left/Right não tentar trocar de aba
	// quando não há abas pra trocar.
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

// showContainer troca pro painel de 4 abas e atualiza seu conteúdo. Usado
// quando a seleção é um Service ou Standalone.
func (dp *detailPanel) showContainer(logs, stats, config, top string) {
	dp.tabs.setContent(logs, stats, config, top)
	dp.resourceActive = false
	dp.root.SwitchToPage(detailPageContainer)
}

// updateStats reescreve só a aba Stats do painel de container. Chamado a
// cada amostra do stream de stats — não mexe em qual página está visível.
func (dp *detailPanel) updateStats(text string) {
	dp.tabs.setStats(text)
}

// showResource troca pro painel simples de Config. Usado quando a seleção
// é uma Image ou um Volume.
func (dp *detailPanel) showResource(text string) {
	dp.configView.SetText(text)
	dp.resourceActive = true
	dp.root.SwitchToPage(detailPageResource)
}

// prevTab/nextTab trocam a aba ativa do painel de container. Não fazem
// nada se o painel de Config (imagem/volume) é o que está visível — não há
// abas pra trocar ali.
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
