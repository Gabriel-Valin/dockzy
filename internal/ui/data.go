package ui

import "github.com/Gabriel-Valin/dockzy/internal/docker"

// Data reúne tudo que o Dashboard precisa desenhar. Services, Standalone,
// Images e Volumes vêm da API real do Docker (ver internal/docker.Client).
// Logs/Stats/Config/Top começam com um placeholder e são preenchidos por
// container conforme a seleção muda (ver Dashboard.OnSelectContainer /
// SetContainerInfo).
type Data struct {
	StatusTitle string
	Services    []docker.Service
	Standalone  []docker.Standalone
	Images      []docker.Image
	Volumes     []docker.Volume
	Logs        string // conteúdo da aba Logs
	Stats       string // conteúdo da aba Stats
	Config      string // conteúdo da aba Container Config
	Top         string // conteúdo da aba Top
}
