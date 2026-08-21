package docker

import (
	"context"
	"strings"

	"github.com/moby/moby/client"
)

type Project struct {
	Name       string
	WorkingDir string
}

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

func (c *Client) ListProjectVolumes(ctx context.Context, project string) ([]Volume, error) {
	result, err := c.VolumeList(ctx, client.VolumeListOptions{
		Filters: make(client.Filters).Add("label", "com.docker.compose.project="+project),
	})
	if err != nil {
		return nil, err
	}
	return volumesFromItems(result.Items), nil
}
