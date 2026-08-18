// Package app é a raiz de composição do dockzy: cria o cliente Docker,
// carrega os dados iniciais, monta o dashboard e liga o stream de CPU até
// a UI. main.go só chama app.Run().
package app

import (
	"context"
	"sync"

	"github.com/Gabriel-Valin/dockzy/internal/docker"
	"github.com/Gabriel-Valin/dockzy/internal/ui"
)

// Run inicializa o cliente Docker, monta o dashboard e roda a aplicação até
// o usuário sair ('q').
func Run() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cli, err := docker.New()
	if err != nil {
		return err
	}
	defer cli.Close()

	services, standalone, err := cli.ListContainers(ctx)
	if err != nil {
		return err
	}

	images, err := cli.ListImages(ctx)
	if err != nil {
		return err
	}

	volumes, err := cli.ListVolumes(ctx)
	if err != nil {
		return err
	}

	const loading = "carregando...\n"
	data := ui.Data{
		StatusTitle: "lazydocker",
		Services:    services,
		Standalone:  standalone,
		Images:      images,
		Volumes:     volumes,
		Logs:        loading,
		Stats:       loading,
		Config:      loading,
		Top:         loading,
	}

	dashboard := ui.New(data, cancel)

	// --- CPU ao vivo: uma goroutine de stream por container rodando,
	// convergindo num único channel consumido pela UI. ---
	cpuUpdates := make(chan docker.CPUUpdate, len(services)+len(standalone))
	for _, id := range dashboard.RunningContainerIDs() {
		go cli.StreamCPU(ctx, id, cpuUpdates)
	}

	go func() {
		for {
			select {
			case u := <-cpuUpdates:
				dashboard.UpdateCPU(u.ID, u.CPU)
			case <-ctx.Done():
				return
			}
		}
	}()

	// --- Painel direito ao vivo: busca info (container, imagem ou volume)
	// da seleção atual em Services/Standalone/Images/Volumes — as quatro
	// escrevem no mesmo painel direito. Uma nova seleção, de qualquer uma
	// delas, cancela a busca anterior ainda em andamento, pra uma resposta
	// lenta não sobrescrever com dados velhos o que já está na tela. ---
	var (
		selectMu     sync.Mutex
		selectCancel context.CancelFunc
	)
	newSelection := func() context.Context {
		selectMu.Lock()
		defer selectMu.Unlock()
		if selectCancel != nil {
			selectCancel()
		}
		selCtx, selCancel := context.WithCancel(ctx)
		selectCancel = selCancel
		return selCtx
	}

	onSelectContainer := func(id string) {
		selCtx := newSelection()

		go func() {
			info, err := cli.Info(selCtx, id)
			if err != nil || selCtx.Err() != nil {
				return
			}
			dashboard.SetContainerInfo(info.Logs, info.Stats, info.Config, info.Top)
		}()

		// Stats ao vivo: enquanto esse container continuar selecionado, uma
		// amostra por segundo reescreve só a aba Stats. selCtx é cancelado
		// (acima) assim que outra seleção acontece, o que encerra tanto
		// StreamStats quanto este consumidor.
		statsUpdates := make(chan docker.StatsUpdate, 1)
		go cli.StreamStats(selCtx, id, statsUpdates)
		go func() {
			for {
				select {
				case u := <-statsUpdates:
					if selCtx.Err() != nil {
						return
					}
					dashboard.UpdateStats(u.Text)
				case <-selCtx.Done():
					return
				}
			}
		}()
	}
	dashboard.OnSelectContainer(onSelectContainer)

	// Imagens e volumes não têm logs/stats/top — só a config (inspect) faz
	// sentido pra eles, daí o painel direito trocar pro modo simples de
	// Config (SetResourceInfo) em vez do modo de 4 abas.
	onSelectImage := func(id string) {
		selCtx := newSelection()
		go func() {
			text, err := cli.ImageInfo(selCtx, id)
			if err != nil || selCtx.Err() != nil {
				return
			}
			dashboard.SetResourceInfo(text)
		}()
	}
	dashboard.OnSelectImage(onSelectImage)

	onSelectVolume := func(name string) {
		selCtx := newSelection()
		go func() {
			text, err := cli.VolumeInfo(selCtx, name)
			if err != nil || selCtx.Err() != nil {
				return
			}
			dashboard.SetResourceInfo(text)
		}()
	}
	dashboard.OnSelectVolume(onSelectVolume)

	if len(services) > 0 {
		onSelectContainer(services[0].ID)
	} else if len(standalone) > 0 {
		onSelectContainer(standalone[0].ID)
	}

	return dashboard.Run()
}
