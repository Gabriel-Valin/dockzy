package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type liveRow struct {
	list  *tview.List
	index int
	state string
	name  string
}

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

func New(data Data, onQuit func()) *Dashboard {
	app := tview.NewApplication()

	statusBox := buildStatusBox(data)
	servicesBox := buildServicesBox(data)
	standaloneBox := buildStandaloneBox(data)
	imagesBox := buildImagesBox(data)
	volumesBox := buildVolumesBox(data)

	leftPanel := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(statusBox, 3, 0, false).
		AddItem(servicesBox, 6, 0, true).
		AddItem(standaloneBox, 5, 0, false).
		AddItem(imagesBox, 6, 0, false).
		AddItem(volumesBox, 6, 0, false)

	rows := map[string]*liveRow{}
	for i, s := range data.Services {
		rows[s.ID] = &liveRow{list: servicesBox, index: i, state: s.State, name: s.Name}
	}
	for i, s := range data.Standalone {
		rows[s.ID] = &liveRow{list: standaloneBox, index: i, state: s.State, name: s.Name}
	}

	detail := newDetailPanel(data)

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

	applyFocusBorder := func(box *tview.Box) {
		box.SetFocusFunc(func() { box.SetBorderColor(colorFocused) })
		box.SetBlurFunc(func() { box.SetBorderColor(colorUnfocused) })
	}
	applyFocusBorder(detail.root.Box)

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
	})
	servicesBox.SetBlurFunc(func() { servicesBox.SetBorderColor(colorUnfocused) })

	standaloneBox.SetFocusFunc(func() {
		standaloneBox.SetBorderColor(colorFocused)
	})
	standaloneBox.SetBlurFunc(func() { standaloneBox.SetBorderColor(colorUnfocused) })

	imagesBox.SetFocusFunc(func() {
		imagesBox.SetBorderColor(colorFocused)
	})
	imagesBox.SetBlurFunc(func() { imagesBox.SetBorderColor(colorUnfocused) })

	volumesBox.SetFocusFunc(func() {
		volumesBox.SetBorderColor(colorFocused)
	})
	volumesBox.SetBlurFunc(func() { volumesBox.SetBorderColor(colorUnfocused) })

	const tabsFocusIdx = 4

	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyTab:
			d.focusIdx = (d.focusIdx + 1) % len(d.focusables)
			app.SetFocus(d.focusables[d.focusIdx])
			return nil
		case tcell.KeyBacktab:
			d.focusIdx = (d.focusIdx - 1 + len(d.focusables)) % len(d.focusables)
			app.SetFocus(d.focusables[d.focusIdx])
			return nil
		}

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

		if event.Rune() == 'q' {
			if onQuit != nil {
				onQuit()
			}
			app.Stop()
			return nil
		}
		return event
	})

	app.SetFocus(servicesBox)

	return d
}

func (d *Dashboard) OnSelectContainer(fn func(id string)) {
	d.onSelectContainer = fn
}

func (d *Dashboard) OnSelectImage(fn func(id string)) {
	d.onSelectImage = fn
}

func (d *Dashboard) OnSelectVolume(fn func(name string)) {
	d.onSelectVolume = fn
}

func (d *Dashboard) RunningContainerIDs() []string {
	ids := make([]string, 0, len(d.rows))
	for id, row := range d.rows {
		if row.state == "running" {
			ids = append(ids, id)
		}
	}
	return ids
}

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

func (d *Dashboard) SetContainerInfo(logs, stats, config, top string) {
	d.App.QueueUpdateDraw(func() {
		d.detail.showContainer(logs, stats, config, top)
	})
}

func (d *Dashboard) UpdateStats(text string) {
	d.App.QueueUpdateDraw(func() {
		d.detail.updateStats(text)
	})
}

func (d *Dashboard) SetResourceInfo(text string) {
	d.App.QueueUpdateDraw(func() {
		d.detail.showResource(text)
	})
}

func (d *Dashboard) Run() error {
	return d.App.SetRoot(d.root, true).EnableMouse(true).Run()
}
