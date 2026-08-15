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

	const loading = "carregando...\n"
	data := ui.Data{
		StatusTitle: "lazydocker",
		Services:    services,
		Standalone:  standalone,
		Images:      docker.MockImages(),
		Volumes:     docker.MockVolumes(),
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

	// --- Painel direito ao vivo: busca logs/stats/config/top do container
	// selecionado. Uma nova seleção cancela a busca anterior ainda em
	// andamento, pra uma resposta lenta não sobrescrever com dados velhos
	// o que já está na tela. ---
	var (
		selectMu     sync.Mutex
		selectCancel context.CancelFunc
	)
	onSelectContainer := func(id string) {
		selectMu.Lock()
		if selectCancel != nil {
			selectCancel()
		}
		selCtx, selCancel := context.WithCancel(ctx)
		selectCancel = selCancel
		selectMu.Unlock()

		go func() {
			info, err := cli.Info(selCtx, id)
			if err != nil || selCtx.Err() != nil {
				return
			}
			dashboard.SetContainerInfo(info.Logs, info.Stats, info.Config, info.Top)
		}()
	}
	dashboard.OnSelectContainer(onSelectContainer)

	if len(services) > 0 {
		onSelectContainer(services[0].ID)
	} else if len(standalone) > 0 {
		onSelectContainer(standalone[0].ID)
	}

	return dashboard.Run()
}
