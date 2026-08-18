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

	root   *tview.Flex
	detail *detailPanel
	rows   map[string]*liveRow

	focusables []tview.Primitive
	focusIdx   int

	onSelectContainer func(id string)
	onSelectImage     func(id string)
	onSelectVolume    func(name string)
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

	// --- Painel direito: abas de container, ou Config de imagem/volume ---
	detail := newDetailPanel(data)

	// --- Layout raiz: esquerda (peso 1) + direita (peso 2) ---
	rootFlex := tview.NewFlex().
		AddItem(leftPanel, 0, 1, true).
		AddItem(detail.root, 0, 2, false)

	d := &Dashboard{
		App:    app,
		root:   rootFlex,
		detail: detail,
		rows:   rows,
		focusables: []tview.Primitive{
			servicesBox, standaloneBox, imagesBox, volumesBox, detail.root,
		},
	}

	// Aplica realce de borda em foco/blur pra cada caixa navegável.
	applyFocusBorder := func(box *tview.Box) {
		box.SetFocusFunc(func() { box.SetBorderColor(colorFocused) })
		box.SetBlurFunc(func() { box.SetBorderColor(colorUnfocused) })
	}
	applyFocusBorder(detail.root.Box)

	// Services, Standalone, Images e Volumes disparam sua respectiva
	// onSelect*: ao navegar (SetChangedFunc) e ao ganhar foco (pra
	// atualizar o painel direito mesmo se o item em destaque não mudou de
	// índice).
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
	selectImages := func() {
		if d.onSelectImage == nil || len(data.Images) == 0 {
			return
		}
		if idx := imagesBox.GetCurrentItem(); idx >= 0 && idx < len(data.Images) {
			d.onSelectImage(data.Images[idx].ID)
		}
	}
	selectVolumes := func() {
		if d.onSelectVolume == nil || len(data.Volumes) == 0 {
			return
		}
		if idx := volumesBox.GetCurrentItem(); idx >= 0 && idx < len(data.Volumes) {
			d.onSelectVolume(data.Volumes[idx].Name)
		}
	}

	servicesBox.SetChangedFunc(func(index int, mainText, secondaryText string, shortcut rune) {
		selectServices()
	})
	standaloneBox.SetChangedFunc(func(index int, mainText, secondaryText string, shortcut rune) {
		selectStandalone()
	})
	imagesBox.SetChangedFunc(func(index int, mainText, secondaryText string, shortcut rune) {
		selectImages()
	})
	volumesBox.SetChangedFunc(func(index int, mainText, secondaryText string, shortcut rune) {
		selectVolumes()
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

	imagesBox.SetFocusFunc(func() {
		imagesBox.SetBorderColor(colorFocused)
		selectImages()
	})
	imagesBox.SetBlurFunc(func() { imagesBox.SetBorderColor(colorUnfocused) })

	volumesBox.SetFocusFunc(func() {
		volumesBox.SetBorderColor(colorFocused)
		selectVolumes()
	})
	volumesBox.SetBlurFunc(func() { volumesBox.SetBorderColor(colorUnfocused) })

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
				detail.prevTab()
				return nil
			case tcell.KeyRight:
				detail.nextTab()
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

// OnSelectImage registra fn pra ser chamada com o ID da imagem em destaque
// sempre que a seleção mudar em Images. Deve ser chamado antes de Run.
func (d *Dashboard) OnSelectImage(fn func(id string)) {
	d.onSelectImage = fn
}

// OnSelectVolume registra fn pra ser chamada com o nome do volume em
// destaque sempre que a seleção mudar em Volumes. Deve ser chamado antes de
// Run.
func (d *Dashboard) OnSelectVolume(fn func(name string)) {
	d.onSelectVolume = fn
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

// SetContainerInfo troca o painel direito pro modo de 4 abas (Logs, Stats,
// Container Config, Top) e atualiza seu conteúdo. Chamado ao selecionar um
// Service ou Standalone. Seguro pra chamar de qualquer goroutine.
func (d *Dashboard) SetContainerInfo(logs, stats, config, top string) {
	d.App.QueueUpdateDraw(func() {
		d.detail.showContainer(logs, stats, config, top)
	})
}

// UpdateStats reescreve só a aba Stats do painel de container (CPU,
// memória, rede, disco, PIDs), sem mexer em Logs/Config/Top. Chamado a cada
// amostra do stream de stats do container selecionado. Seguro pra chamar
// de qualquer goroutine.
func (d *Dashboard) UpdateStats(text string) {
	d.App.QueueUpdateDraw(func() {
		d.detail.updateStats(text)
	})
}

// SetResourceInfo troca o painel direito pro modo simples de Config (sem
// Logs/Stats/Top) e mostra text. Chamado ao selecionar uma Image ou um
// Volume, que não têm logs, stats ao vivo nem processos "top". Seguro pra
// chamar de qualquer goroutine.
func (d *Dashboard) SetResourceInfo(text string) {
	d.App.QueueUpdateDraw(func() {
		d.detail.showResource(text)
	})
}

// Run desenha a tela e bloqueia até o usuário sair ('q').
func (d *Dashboard) Run() error {
	return d.App.SetRoot(d.root, true).EnableMouse(true).Run()
}
