package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// liveRow localiza a linha de um container numa das listas da esquerda, pra
// dar pra reescrevê-la quando chegar uma nova amostra de CPU.
type liveRow struct {
	list  *tview.List
	index int
	state string
	name  string
}

// Dashboard monta e controla a tela inteira: painel esquerdo (status,
// services, standalone, images, volumes), painel direito (abas) e o foco
// entre eles.
type Dashboard struct {
	App *tview.Application

	root *tview.Flex
	tabs *tabbedPanel
	rows map[string]*liveRow

	focusables []tview.Primitive
	focusIdx   int

	onSelectContainer func(id string)
}

// New monta o dashboard a partir dos dados iniciais. onQuit é chamado antes
// de a aplicação parar (ex: pra cancelar streams de CPU em andamento). Use
// OnSelectContainer pra reagir à seleção de um container em Services ou
// Standalone.
func New(data Data, onQuit func()) *Dashboard {
	app := tview.NewApplication()

	// --- Painel esquerdo: 5 caixas empilhadas ---
	statusBox := buildStatusBox(data)
	servicesBox := buildServicesBox(data)
	standaloneBox := buildStandaloneBox(data)
	imagesBox := buildImagesBox(data)
	volumesBox := buildVolumesBox(data)

	// Alturas fixas pras caixas pequenas; Services e Volumes crescem.
	// Os números são: altura em linhas (0 = flexível, com peso).
	leftPanel := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(statusBox, 3, 0, false).  // 1 linha + bordas
		AddItem(servicesBox, 6, 0, true). // recebe foco inicial
		AddItem(standaloneBox, 5, 0, false).
		AddItem(imagesBox, 6, 0, false).
		AddItem(volumesBox, 6, 0, false)

	// --- CPU ao vivo: uma linha por container, atualizada por stream ---
	rows := map[string]*liveRow{}
	for i, s := range data.Services {
		rows[s.ID] = &liveRow{list: servicesBox, index: i, state: s.State, name: s.Name}
	}
	for i, s := range data.Standalone {
		rows[s.ID] = &liveRow{list: standaloneBox, index: i, state: s.State, name: s.Name}
	}

	// --- Painel direito: abas ---
	tabs := newTabbedPanel(data)

	// --- Layout raiz: esquerda (peso 1) + direita (peso 2) ---
	rootFlex := tview.NewFlex().
		AddItem(leftPanel, 0, 1, true).
		AddItem(tabs.root, 0, 2, false)

	d := &Dashboard{
		App:  app,
		root: rootFlex,
		tabs: tabs,
		rows: rows,
		focusables: []tview.Primitive{
			servicesBox, standaloneBox, imagesBox, volumesBox, tabs.root,
		},
	}

	// Aplica realce de borda em foco/blur pra cada caixa navegável.
	applyFocusBorder := func(box *tview.Box) {
		box.SetFocusFunc(func() { box.SetBorderColor(colorFocused) })
		box.SetBlurFunc(func() { box.SetBorderColor(colorUnfocused) })
	}
	applyFocusBorder(imagesBox.Box)
	applyFocusBorder(volumesBox.Box)
	applyFocusBorder(tabs.root.Box)

	// Services e Standalone também disparam d.onSelectContainer: ao navegar
	// (SetChangedFunc) e ao ganhar foco (pra atualizar o painel direito
	// mesmo se o item em destaque não mudou de índice).
	selectServices := func() {
		if d.onSelectContainer == nil || len(data.Services) == 0 {
			return
		}
		if idx := servicesBox.GetCurrentItem(); idx >= 0 && idx < len(data.Services) {
			d.onSelectContainer(data.Services[idx].ID)
		}
	}
	selectStandalone := func() {
		if d.onSelectContainer == nil || len(data.Standalone) == 0 {
			return
		}
		if idx := standaloneBox.GetCurrentItem(); idx >= 0 && idx < len(data.Standalone) {
			d.onSelectContainer(data.Standalone[idx].ID)
		}
	}

	servicesBox.SetChangedFunc(func(index int, mainText, secondaryText string, shortcut rune) {
		selectServices()
	})
	standaloneBox.SetChangedFunc(func(index int, mainText, secondaryText string, shortcut rune) {
		selectStandalone()
	})

	servicesBox.SetFocusFunc(func() {
		servicesBox.SetBorderColor(colorFocused)
		selectServices()
	})
	servicesBox.SetBlurFunc(func() { servicesBox.SetBorderColor(colorUnfocused) })

	standaloneBox.SetFocusFunc(func() {
		standaloneBox.SetBorderColor(colorFocused)
		selectStandalone()
	})
	standaloneBox.SetBlurFunc(func() { standaloneBox.SetBorderColor(colorUnfocused) })

	// Índice do painel de abas em focusables, pra saber quando as setas
	// devem trocar de aba em vez de navegar dentro de uma lista.
	const tabsFocusIdx = 4

	// --- Teclas globais ---
	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyTab:
			d.focusIdx = (d.focusIdx + 1) % len(d.focusables)
			app.SetFocus(d.focusables[d.focusIdx])
			return nil
		case tcell.KeyBacktab: // Shift+Tab
			d.focusIdx = (d.focusIdx - 1 + len(d.focusables)) % len(d.focusables)
			app.SetFocus(d.focusables[d.focusIdx])
			return nil
		}

		// Setas esquerda/direita trocam de aba QUANDO o painel direito
		// está focado.
		if d.focusIdx == tabsFocusIdx {
			switch event.Key() {
			case tcell.KeyLeft:
				tabs.prev()
				return nil
			case tcell.KeyRight:
				tabs.next()
				return nil
			}
		}

		// 'q' sai: dá chance de cancelar streams em andamento antes de
		// derrubar a UI.
		if event.Rune() == 'q' {
			if onQuit != nil {
				onQuit()
			}
			app.Stop()
			return nil
		}
		return event
	})

	// Força o FocusFunc a rodar no painel inicial (borda verde de largada).
	app.SetFocus(servicesBox)

	return d
}

// OnSelectContainer registra fn pra ser chamada com o ID do container em
// destaque sempre que a seleção mudar em Services ou Standalone. Deve ser
// chamado antes de Run.
func (d *Dashboard) OnSelectContainer(fn func(id string)) {
	d.onSelectContainer = fn
}

// RunningContainerIDs devolve os IDs dos containers que estão rodando —
// os únicos candidatos a stream de CPU.
func (d *Dashboard) RunningContainerIDs() []string {
	ids := make([]string, 0, len(d.rows))
	for id, row := range d.rows {
		if row.state == "running" {
			ids = append(ids, id)
		}
	}
	return ids
}

// UpdateCPU reescreve a linha de um container com o novo valor de CPU.
// Seguro pra chamar de qualquer goroutine.
func (d *Dashboard) UpdateCPU(id, cpu string) {
	row, ok := d.rows[id]
	if !ok {
		return
	}
	d.App.QueueUpdateDraw(func() {
		text := formatRow(row.state, row.name, cpu, stateColor(row.state))
		row.list.SetItemText(row.index, text, "")
	})
}

// SetContainerInfo atualiza o conteúdo das quatro abas do painel direito
// (Logs, Stats, Container Config, Top). Seguro pra chamar de qualquer
// goroutine.
func (d *Dashboard) SetContainerInfo(logs, stats, config, top string) {
	d.App.QueueUpdateDraw(func() {
		d.tabs.setContent(logs, stats, config, top)
	})
}

// Run desenha a tela e bloqueia até o usuário sair ('q').
func (d *Dashboard) Run() error {
	return d.App.SetRoot(d.root, true).EnableMouse(true).Run()
}
