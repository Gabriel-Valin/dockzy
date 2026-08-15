// Package docker fala com o daemon do Docker: lista containers e assina o
// stream de stats pra calcular CPU ao vivo. É a única camada do projeto que
// conhece a API do Docker — main/app e ui não importam client diretamente.
package docker

import (
	"context"
	"strings"

	"github.com/moby/moby/client"
)

// Service = container gerenciado por docker-compose (tem nome de serviço).
type Service struct {
	State string // "running", "exited" etc. -> Container.State
	Name  string // nome do serviço            -> label com.docker.compose.service
	CPU   string // "0.03%"                     -> stream de stats (calculado)
	ID    string // usado depois pra logs/stats -> Container.ID
}

// Standalone = container avulso (não faz parte de um compose).
type Standalone struct {
	State string
	Name  string
	CPU   string
	ID    string
}

// Client é um client do Docker (github.com/moby/moby/client) com os
// métodos usados pelo dockzy.
type Client struct {
	*client.Client
}

// New abre uma conexão com o Docker usando as variáveis de ambiente padrão
// (DOCKER_HOST, DOCKER_CERT_PATH etc.).
func New() (*Client, error) {
	cli, err := client.New(client.FromEnv)
	if err != nil {
		return nil, err
	}
	return &Client{cli}, nil
}

// ListContainers lista todos os containers (rodando ou não) e os separa em
// Services (têm o label do compose) e Standalone (containers avulsos). CPU
// começa em "0.00%" pra cada um e é atualizado depois via StreamCPU.
func (c *Client) ListContainers(ctx context.Context) (services []Service, standalone []Standalone, err error) {
	result, err := c.ContainerList(ctx, client.ContainerListOptions{All: true})
	if err != nil {
		return nil, nil, err
	}

	for _, item := range result.Items {
		state := string(item.State)

		if svc, ok := item.Labels["com.docker.compose.service"]; ok {
			services = append(services, Service{State: state, Name: svc, CPU: "0.00%", ID: item.ID})
			continue
		}

		name := strings.TrimPrefix(firstName(item.Names), "/")
		standalone = append(standalone, Standalone{State: state, Name: name, CPU: "0.00%", ID: item.ID})
	}
	return services, standalone, nil
}

func firstName(names []string) string {
	if len(names) == 0 {
		return ""
	}
	return names[0]
}
