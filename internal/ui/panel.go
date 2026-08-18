package ui

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// ============================================================================
// PAINEL ESQUERDO
//
// São 5 caixas empilhadas verticalmente. Cada uma é um tview.List (pra ser
// navegável e selecionável) ou TextView. Uso List nas que têm itens
// selecionáveis e TextView nas puramente informativas.
// ============================================================================

func buildStatusBox(data Data) *tview.TextView {
	tv := tview.NewTextView().SetDynamicColors(true)
	tv.SetText(data.StatusTitle)
	tv.SetBorder(true).SetTitle(" Status ").SetTitleAlign(tview.AlignLeft)
	return tv
}

// formatRow alinha "estado ... nome ... cpu" numa largura fixa, imitando
// as colunas da screenshot. É simples: pad manual com espaços.
func formatRow(state, name, cpu string, stateColor string) string {
	// Nome ocupa uma coluna de ~16, CPU alinhado à direita.
	namePadded := name
	if len(namePadded) > 16 {
		namePadded = namePadded[:16]
	} else {
		namePadded = namePadded + strings.Repeat(" ", 16-len(namePadded))
	}
	return fmt.Sprintf("[%s]%s[white] %s%s", stateColor, state, namePadded, cpu)
}

// stateColor decide a cor do estado: verde se "running", vermelho caso
// contrário (exited, created, paused, etc.).
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
		// repo (col ~18) + tag (col ~14) + size à direita
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

// applySelectedStyle troca o destaque padrão do tview.List (que inverte
// fundo/texto) por negrito puro, sem fundo colorido — pra não competir com
// as cores de estado (verde/vermelho) que formatRow já embute na linha.
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
