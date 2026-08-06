package main

import (
	"context"
	"fmt"
	"log"

	"github.com/moby/moby/client"
	"github.com/rivo/tview"
)

func main() {
	apiClient, err := client.New(client.FromEnv)
	if err != nil {
		log.Fatal(err)
	}
	defer apiClient.Close()

	containers, err := apiClient.ContainerList(context.Background(), client.ContainerListOptions{
		All: true,
	})
	if err != nil {
		log.Fatal(err)
	}

	app := tview.NewApplication()

	list := tview.NewList().ShowSecondaryText(true)
	list.SetBorder(true).SetTitle(" Containers ")

	details := tview.NewTextView().
		SetDynamicColors(true).
		SetText("Select container...")
	details.SetBorder(true).SetTitle(" Details ")

	for _, c := range containers.Items {
		name := "no-name"
		if len(c.Names) > 0 {
			name = c.Names[0]
		}
		mainText := fmt.Sprintf("%s  [%s]", name[1:], c.State)

		cont := c

		list.AddItem(mainText, "", 0, func() {
			info := fmt.Sprintf(
				"[yellow]Name:[white] %s\n[yellow]Image:[white] %s\n[yellow]State:[white] %s\n[yellow]ID:[white] %s\n\n[gray](stats will come here)",
				name, cont.Image, cont.State, cont.ID[:12],
			)
			details.SetText(info)
		}).ShowSecondaryText(false)
	}

	flex := tview.NewFlex().
		AddItem(list, 0, 1, true).
		AddItem(details, 0, 2, false)

	if err := app.SetRoot(flex, true).Run(); err != nil {
		panic(err)
	}
}
