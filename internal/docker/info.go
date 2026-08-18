package docker

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/moby/moby/api/pkg/stdcopy"
	"github.com/moby/moby/client"
)

// ContainerInfo reúne o que o painel direito (Logs, Stats, Container
// Config, Top) precisa mostrar sobre um container específico.
type ContainerInfo struct {
	Logs   string
	Stats  string
	Config string
	Top    string
}

// Info busca logs, stats, inspect e top de um container e monta o texto de
// cada aba. As quatro chamadas rodam em paralelo pra reduzir a latência
// percebida ao trocar de container selecionado.
func (c *Client) Info(ctx context.Context, id string) (ContainerInfo, error) {
	type result struct {
		field string
		text  string
		err   error
	}

	fields := []string{"logs", "stats", "config", "top"}
	results := make(chan result, len(fields))

	go func() {
		text, err := c.containerLogs(ctx, id)
		results <- result{"logs", text, err}
	}()
	go func() {
		text, err := c.containerStatsSnapshot(ctx, id)
		results <- result{"stats", text, err}
	}()
	go func() {
		text, err := c.containerConfig(ctx, id)
		results <- result{"config", text, err}
	}()
	go func() {
		text, err := c.containerTop(ctx, id)
		results <- result{"top", text, err}
	}()

	var info ContainerInfo
	for range fields {
		r := <-results
		text := r.text
		if r.err != nil {
			text = fmt.Sprintf("erro: %s", r.err)
		}
		switch r.field {
		case "logs":
			info.Logs = text
		case "stats":
			info.Stats = text
		case "config":
			info.Config = text
		case "top":
			info.Top = text
		}
	}
	return info, nil
}

// containerLogs busca as últimas linhas de stdout/stderr do container.
func (c *Client) containerLogs(ctx context.Context, id string) (string, error) {
	result, err := c.ContainerLogs(ctx, id, client.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Tail:       "100",
	})
	if err != nil {
		return "", err
	}
	defer result.Close()

	var out bytes.Buffer
	if _, err := stdcopy.StdCopy(&out, &out, result); err != nil {
		return "", err
	}
	if out.Len() == 0 {
		return "(sem logs)", nil
	}
	return out.String(), nil
}

// containerStatsSnapshot pega uma amostra única de stats (com uma amostra
// anterior pro cálculo de delta de CPU) e formata como no "docker stats".
func (c *Client) containerStatsSnapshot(ctx context.Context, id string) (string, error) {
	result, err := c.ContainerStats(ctx, id, client.ContainerStatsOptions{
		Stream:                false,
		IncludePreviousSample: true,
	})
	if err != nil {
		return "", err
	}
	defer result.Body.Close()

	stats, err := decodeStats(result.Body)
	if err != nil {
		return "", err
	}

	return formatStatsText(stats), nil
}

// containerConfig busca o inspect do container e formata imagem, comando,
// portas publicadas e variáveis de ambiente.
func (c *Client) containerConfig(ctx context.Context, id string) (string, error) {
	result, err := c.ContainerInspect(ctx, id, client.ContainerInspectOptions{})
	if err != nil {
		return "", err
	}
	info := result.Container

	var b strings.Builder
	if info.Config != nil {
		fmt.Fprintf(&b, "Image: %s\n", info.Config.Image)
		if len(info.Config.Cmd) > 0 {
			fmt.Fprintf(&b, "Command: %s\n", strings.Join(info.Config.Cmd, " "))
		}
	}

	if info.NetworkSettings != nil {
		for port, bindings := range info.NetworkSettings.Ports {
			for _, binding := range bindings {
				fmt.Fprintf(&b, "Ports: %s -> %s:%s\n", port.String(), binding.HostIP, binding.HostPort)
			}
		}
	}

	if info.Config != nil && len(info.Config.Env) > 0 {
		b.WriteString("Env:\n")
		for _, env := range info.Config.Env {
			fmt.Fprintf(&b, "  %s\n", env)
		}
	}

	return b.String(), nil
}

// containerTop lista os processos rodando dentro do container.
func (c *Client) containerTop(ctx context.Context, id string) (string, error) {
	result, err := c.ContainerTop(ctx, id, client.ContainerTopOptions{})
	if err != nil {
		return "", err
	}

	var b strings.Builder
	b.WriteString(strings.Join(result.Titles, "\t"))
	b.WriteByte('\n')
	for _, proc := range result.Processes {
		b.WriteString(strings.Join(proc, "\t"))
		b.WriteByte('\n')
	}
	return b.String(), nil
}

// formatBytes converte bytes pra uma unidade legível (kB/MB/GB), no mesmo
// espírito do "docker stats".
func formatBytes(n uint64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%dB", n)
	}
	div, exp := uint64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%ciB", float64(n)/float64(div), "KMGTPE"[exp])
}
