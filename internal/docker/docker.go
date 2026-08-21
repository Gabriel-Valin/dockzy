package docker

import (
	"context"
	"strings"

	"github.com/moby/moby/client"
)

type Service struct {
	State string
	Name  string
	CPU   string
	ID    string
}

type Standalone struct {
	State string
	Name  string
	CPU   string
	ID    string
}

type Client struct {
	*client.Client
}

func New() (*Client, error) {
	cli, err := client.New(client.FromEnv)
	if err != nil {
		return nil, err
	}
	return &Client{cli}, nil
}

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
