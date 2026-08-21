// Package app é a raiz de composição do dockzy: cria o cliente Docker,
// carrega os dados iniciais, monta o dashboard e liga o stream de CPU até
// a UI. main.go só chama app.Run().
package app

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/Gabriel-Valin/dockzy/internal/docker"
	"github.com/Gabriel-Valin/dockzy/internal/ui"
)

// Run inicializa o cliente Docker, monta o dashboard e roda a aplicação até
// o usuário sair ('q'). Se all for true, ignora qualquer projeto
// docker-compose detectado na pasta atual e carrega tudo do host — igual
// ao comportamento de quando nenhum projeto é detectado.
func Run(all bool) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cli, err := docker.New()
	if err != nil {
		return err
	}
	defer cli.Close()

	project, scoped, err := detectProject(ctx, cli, all)
	if err != nil {
		return err
	}

	statusTitle := "lazydocker"
	var services []docker.Service
	var standalone []docker.Standalone
	var images []docker.Image
	var volumes []docker.Volume

	if scoped {
		if services, err = cli.ListProjectContainers(ctx, project.Name); err != nil {
			return err
		}
		if images, err = cli.ListProjectImages(ctx, project.Name); err != nil {
			return err
		}
		if volumes, err = cli.ListProjectVolumes(ctx, project.Name); err != nil {
			return err
		}
		statusTitle = formatProjectStatus(project.Name, services)
	} else {
		if services, standalone, err = cli.ListContainers(ctx); err != nil {
			return err
		}
		if images, err = cli.ListImages(ctx); err != nil {
			return err
		}
		if volumes, err = cli.ListVolumes(ctx); err != nil {
			return err
		}
	}

	const loading = "carregando...\n"
	data := ui.Data{
		StatusTitle: statusTitle,
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

// detectProject decide se a sessão atual deve ser escopada a um projeto
// docker-compose. Se all for true, nunca escopa (usuário pediu
// explicitamente pra ver tudo). Senão, tenta detectar um projeto a partir
// da pasta de onde o dockzy foi chamado.
func detectProject(ctx context.Context, cli *docker.Client, all bool) (docker.Project, bool, error) {
	if all {
		return docker.Project{}, false, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return docker.Project{}, false, err
	}
	return cli.DetectProject(ctx, cwd)
}

// formatProjectStatus monta o texto do painel Status quando a sessão está
// escopada a um projeto docker-compose: nome do projeto e os nomes dos
// serviços relacionados, igual ao lazydocker.
func formatProjectStatus(project string, services []docker.Service) string {
	names := make([]string, len(services))
	for i, s := range services {
		names[i] = s.Name
	}
	return fmt.Sprintf("%s\nServices: %s", project, strings.Join(names, ", "))
}
