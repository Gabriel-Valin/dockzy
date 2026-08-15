package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
)

// CPUUpdate carrega o novo valor de CPU calculado pra um container.
type CPUUpdate struct {
	ID  string
	CPU string
}

// StreamCPU assina o stream de stats do container e manda um CPUUpdate a
// cada amostra recebida (o daemon manda uma por segundo). Encerra quando
// ctx é cancelado, quando o stream fecha, ou quando o container não existe
// mais. Feito pra rodar em goroutine própria, uma por container.
func (c *Client) StreamCPU(ctx context.Context, id string, updates chan<- CPUUpdate) {
	result, err := c.ContainerStats(ctx, id, client.ContainerStatsOptions{Stream: true})
	if err != nil {
		return
	}
	defer result.Body.Close()

	decoder := json.NewDecoder(result.Body)
	for {
		var stats container.StatsResponse
		if err := decoder.Decode(&stats); err != nil {
			return
		}

		select {
		case updates <- CPUUpdate{ID: id, CPU: formatCPUPercent(stats)}:
		case <-ctx.Done():
			return
		}
	}
}

// decodeStats decodifica uma única amostra de [container.StatsResponse] do
// corpo de uma resposta de stats.
func decodeStats(r io.Reader) (container.StatsResponse, error) {
	var stats container.StatsResponse
	if err := json.NewDecoder(r).Decode(&stats); err != nil {
		return container.StatsResponse{}, err
	}
	return stats, nil
}

// formatCPUPercent replica o cálculo do "docker stats": delta de uso de CPU
// do container sobre delta de uso de CPU do sistema, escalado pelo número
// de CPUs online.
func formatCPUPercent(stats container.StatsResponse) string {
	cpuDelta := float64(stats.CPUStats.CPUUsage.TotalUsage) - float64(stats.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(stats.CPUStats.SystemUsage) - float64(stats.PreCPUStats.SystemUsage)
	if cpuDelta <= 0 || systemDelta <= 0 {
		return "0.00%"
	}

	onlineCPUs := float64(stats.CPUStats.OnlineCPUs)
	if onlineCPUs == 0 {
		onlineCPUs = float64(len(stats.CPUStats.CPUUsage.PercpuUsage))
	}
	if onlineCPUs == 0 {
		onlineCPUs = 1
	}

	return fmt.Sprintf("%.2f%%", (cpuDelta/systemDelta)*onlineCPUs*100.0)
}
