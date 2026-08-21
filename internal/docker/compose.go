package docker

import (
	"context"
	"strings"

	"github.com/moby/moby/client"
)

// Project identifica um projeto docker-compose: o nome
// (com.docker.compose.project) e a pasta de onde "docker compose up" foi
// rodado (com.docker.compose.project.working_dir).
type Project struct {
	Name       string
	WorkingDir string
}

// DetectProject procura, entre os containers existentes (rodando ou não),
// um que pertença a um projeto docker-compose subido a partir de
// workingDir. workingDir deve ser um caminho absoluto (ex: os.Getwd()).
// Devolve ok=false se nenhum bate — não dá pra detectar um projeto que
// nunca teve "docker compose up" rodado, só pela API do Docker.
func (c *Client) DetectProject(ctx context.Context, workingDir string) (Project, bool, error) {
	result, err := c.ContainerList(ctx, client.ContainerListOptions{
		All:     true,
		Filters: make(client.Filters).Add("label", "com.docker.compose.project.working_dir="+workingDir),
	})
	if err != nil {
		return Project{}, false, err
	}
	if len(result.Items) == 0 {
		return Project{}, false, nil
	}
	return Project{
		Name:       result.Items[0].Labels["com.docker.compose.project"],
		WorkingDir: workingDir,
	}, true, nil
}

// ListProjectContainers lista os containers de um projeto docker-compose
// (com.docker.compose.project == project). Todos entram como Service —
// "standalone" não é um conceito que existe dentro de um projeto: todo
// container ali tem, por definição, o label com.docker.compose.service.
func (c *Client) ListProjectContainers(ctx context.Context, project string) ([]Service, error) {
	result, err := c.ContainerList(ctx, client.ContainerListOptions{
		All:     true,
		Filters: make(client.Filters).Add("label", "com.docker.compose.project="+project),
	})
	if err != nil {
		return nil, err
	}

	services := make([]Service, 0, len(result.Items))
	for _, item := range result.Items {
		name := item.Labels["com.docker.compose.service"]
		if name == "" {
			name = strings.TrimPrefix(firstName(item.Names), "/")
		}
		services = append(services, Service{State: string(item.State), Name: name, CPU: "0.00%", ID: item.ID})
	}
	return services, nil
}

// ListProjectImages lista as imagens usadas pelos containers de um
// projeto docker-compose. Imagens puxadas de um registry não carregam o
// label com.docker.compose.project (só containers e volumes carregam) —
// por isso o filtro aqui é indireto: pega o ImageID de cada container do
// projeto e casa contra a listagem completa de imagens do host.
func (c *Client) ListProjectImages(ctx context.Context, project string) ([]Image, error) {
	containers, err := c.ContainerList(ctx, client.ContainerListOptions{
		All:     true,
		Filters: make(client.Filters).Add("label", "com.docker.compose.project="+project),
	})
	if err != nil {
		return nil, err
	}

	usedImageIDs := make(map[string]bool, len(containers.Items))
	for _, item := range containers.Items {
		usedImageIDs[item.ImageID] = true
	}

	result, err := c.ImageList(ctx, client.ImageListOptions{All: false})
	if err != nil {
		return nil, err
	}

	var images []Image
	for _, summary := range result.Items {
		if !usedImageIDs[summary.ID] {
			continue
		}
		images = append(images, imagesFromSummary(summary)...)
	}
	return images, nil
}

// ListProjectVolumes lista os volumes de um projeto docker-compose
// (com.docker.compose.project == project) — compose marca os volumes
// nomeados que ele cria com esse label.
func (c *Client) ListProjectVolumes(ctx context.Context, project string) ([]Volume, error) {
	result, err := c.VolumeList(ctx, client.VolumeListOptions{
		Filters: make(client.Filters).Add("label", "com.docker.compose.project="+project),
	})
	if err != nil {
		return nil, err
	}
	return volumesFromItems(result.Items), nil
}
