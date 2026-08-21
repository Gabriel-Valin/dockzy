package ui

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func buildStatusBox(data Data) *tview.TextView {
	tv := tview.NewTextView().SetDynamicColors(true)
	tv.SetText(data.StatusTitle)
	tv.SetBorder(true).SetTitle(" Status ").SetTitleAlign(tview.AlignLeft)
	return tv
}

func formatRow(state, name, cpu string, stateColor string) string {
	namePadded := name
	if len(namePadded) > 16 {
		namePadded = namePadded[:16]
	} else {
		namePadded = namePadded + strings.Repeat(" ", 16-len(namePadded))
	}
	return fmt.Sprintf("[%s]%s[white] %s%s", stateColor, state, namePadded, cpu)
}

func stateColor(state string) string {
	if strings.HasPrefix(state, "running") {
		return "green"
	}
	return "red"
}

func buildServicesBox(data Data) *tview.List {
	list := tview.NewList().ShowSecondaryText(false)
	list.SetBorder(true).SetTitle(" Services ").SetTitleAlign(tview.AlignLeft)
	applySelectedStyle(list)

	for _, s := range data.Services {
		row := formatRow(s.State, s.Name, s.CPU, stateColor(s.State))
		list.AddItem(row, "", 0, nil)
	}
	return list
}

func buildStandaloneBox(data Data) *tview.List {
	list := tview.NewList().ShowSecondaryText(false)
	list.SetBorder(true).SetTitle(" Standalone Containers ").SetTitleAlign(tview.AlignLeft)
	applySelectedStyle(list)

	for _, s := range data.Standalone {
		row := formatRow(s.State, s.Name, s.CPU, stateColor(s.State))
		list.AddItem(row, "", 0, nil)
	}
	return list
}

func buildImagesBox(data Data) *tview.List {
	list := tview.NewList().ShowSecondaryText(false)
	list.SetBorder(true).SetTitle(" Images ").SetTitleAlign(tview.AlignLeft)
	applySelectedStyle(list)

	for _, img := range data.Images {
		repo := padRight(img.Repo, 18)
		tag := padRight(img.Tag, 14)
		row := fmt.Sprintf("%s%s%s", repo, tag, img.Size)
		list.AddItem(row, "", 0, nil)
	}
	return list
}

func buildVolumesBox(data Data) *tview.List {
	list := tview.NewList().ShowSecondaryText(false)
	list.SetBorder(true).SetTitle(" Volumes ").SetTitleAlign(tview.AlignLeft)
	applySelectedStyle(list)

	for _, v := range data.Volumes {
		row := fmt.Sprintf("%s %s", padRight(v.Driver, 6), v.Name)
		list.AddItem(row, "", 0, nil)
	}
	return list
}

func applySelectedStyle(list *tview.List) {
	list.SetSelectedStyle(tcell.StyleDefault.
		Foreground(tview.Styles.PrimaryTextColor).
		Background(tview.Styles.PrimitiveBackgroundColor).
		Bold(true))
}

func padRight(s string, width int) string {
	if len(s) >= width {
		return s[:width]
	}
	return s + strings.Repeat(" ", width-len(s))
}
