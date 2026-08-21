package docker

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/moby/moby/api/types/image"
	"github.com/moby/moby/api/types/volume"
	"github.com/moby/moby/client"
)

type Image struct {
	Repo string
	Tag  string
	Size string
	ID   string
}

type Volume struct {
	Driver string
	Name   string
}

func (c *Client) ListImages(ctx context.Context) ([]Image, error) {
	result, err := c.ImageList(ctx, client.ImageListOptions{All: false})
	if err != nil {
		return nil, err
	}

	var images []Image
	for _, summary := range result.Items {
		images = append(images, imagesFromSummary(summary)...)
	}
	return images, nil
}

func imagesFromSummary(summary image.Summary) []Image {
	id := strings.TrimPrefix(summary.ID, "sha256:")
	if len(id) > 12 {
		id = id[:12]
	}
	size := formatBytes(uint64(summary.Size))

	if len(summary.RepoTags) == 0 {
		return []Image{{Repo: "<none>", Tag: "<none>", Size: size, ID: id}}
	}
	images := make([]Image, 0, len(summary.RepoTags))
	for _, repoTag := range summary.RepoTags {
		repo, tag := splitRepoTag(repoTag)
		images = append(images, Image{Repo: repo, Tag: tag, Size: size, ID: id})
	}
	return images
}

func splitRepoTag(repoTag string) (repo, tag string) {
	tagIdx := strings.LastIndex(repoTag, ":")
	slashIdx := strings.LastIndex(repoTag, "/")
	if tagIdx <= slashIdx {
		return repoTag, "<none>"
	}
	return repoTag[:tagIdx], repoTag[tagIdx+1:]
}

func (c *Client) ListVolumes(ctx context.Context) ([]Volume, error) {
	result, err := c.VolumeList(ctx, client.VolumeListOptions{})
	if err != nil {
		return nil, err
	}
	return volumesFromItems(result.Items), nil
}

func volumesFromItems(items []volume.Volume) []Volume {
	volumes := make([]Volume, 0, len(items))
	for _, v := range items {
		volumes = append(volumes, Volume{Driver: v.Driver, Name: v.Name})
	}
	return volumes
}

func (c *Client) ImageInfo(ctx context.Context, id string) (string, error) {
	result, err := c.ImageInspect(ctx, id)
	if err != nil {
		return "", err
	}
	info := result.InspectResponse

	var b strings.Builder
	fmt.Fprintf(&b, "ID: %s\n", strings.TrimPrefix(info.ID, "sha256:"))
	if len(info.RepoTags) > 0 {
		fmt.Fprintf(&b, "Tags: %s\n", strings.Join(info.RepoTags, ", "))
	}
	if info.Created != "" {
		fmt.Fprintf(&b, "Created: %s\n", info.Created)
	}
	fmt.Fprintf(&b, "Size: %s\n", formatBytes(uint64(info.Size)))
	fmt.Fprintf(&b, "Platform: %s/%s\n", info.Os, info.Architecture)

	if cfg := info.Config; cfg != nil {
		if cfg.WorkingDir != "" {
			fmt.Fprintf(&b, "WorkingDir: %s\n", cfg.WorkingDir)
		}
		if len(cfg.Entrypoint) > 0 {
			fmt.Fprintf(&b, "Entrypoint: %s\n", strings.Join(cfg.Entrypoint, " "))
		}
		if len(cfg.Cmd) > 0 {
			fmt.Fprintf(&b, "Cmd: %s\n", strings.Join(cfg.Cmd, " "))
		}
		if len(cfg.ExposedPorts) > 0 {
			ports := make([]string, 0, len(cfg.ExposedPorts))
			for port := range cfg.ExposedPorts {
				ports = append(ports, port)
			}
			sort.Strings(ports)
			fmt.Fprintf(&b, "Exposed ports: %s\n", strings.Join(ports, ", "))
		}
		if len(cfg.Env) > 0 {
			b.WriteString("Env:\n")
			for _, env := range cfg.Env {
				fmt.Fprintf(&b, "  %s\n", env)
			}
		}
	}

	return b.String(), nil
}

func (c *Client) VolumeInfo(ctx context.Context, name string) (string, error) {
	result, err := c.VolumeInspect(ctx, name, client.VolumeInspectOptions{})
	if err != nil {
		return "", err
	}
	v := result.Volume

	var b strings.Builder
	fmt.Fprintf(&b, "Name: %s\n", v.Name)
	fmt.Fprintf(&b, "Driver: %s\n", v.Driver)
	fmt.Fprintf(&b, "Mountpoint: %s\n", v.Mountpoint)
	if v.Scope != "" {
		fmt.Fprintf(&b, "Scope: %s\n", v.Scope)
	}
	if v.CreatedAt != "" {
		fmt.Fprintf(&b, "Created: %s\n", v.CreatedAt)
	}

	if len(v.Labels) > 0 {
		b.WriteString("Labels:\n")
		for _, k := range sortedKeys(v.Labels) {
			fmt.Fprintf(&b, "  %s=%s\n", k, v.Labels[k])
		}
	}
	if len(v.Options) > 0 {
		b.WriteString("Options:\n")
		for _, k := range sortedKeys(v.Options) {
			fmt.Fprintf(&b, "  %s=%s\n", k, v.Options[k])
		}
	}

	return b.String(), nil
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
