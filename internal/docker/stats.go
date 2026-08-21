package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
)

type CPUUpdate struct {
	ID  string
	CPU string
}

type StatsUpdate struct {
	ID   string
	Text string
}

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

func (c *Client) StreamStats(ctx context.Context, id string, updates chan<- StatsUpdate) {
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
		case updates <- StatsUpdate{ID: id, Text: formatStatsText(stats)}:
		case <-ctx.Done():
			return
		}
	}
}

func decodeStats(r io.Reader) (container.StatsResponse, error) {
	var stats container.StatsResponse
	if err := json.NewDecoder(r).Decode(&stats); err != nil {
		return container.StatsResponse{}, err
	}
	return stats, nil
}

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

func formatStatsText(stats container.StatsResponse) string {
	var rxBytes, txBytes uint64
	for _, net := range stats.Networks {
		rxBytes += net.RxBytes
		txBytes += net.TxBytes
	}

	var readBytes, writeBytes uint64
	for _, entry := range stats.BlkioStats.IoServiceBytesRecursive {
		switch strings.ToLower(entry.Op) {
		case "read":
			readBytes += entry.Value
		case "write":
			writeBytes += entry.Value
		}
	}

	return fmt.Sprintf(
		"CPU:    %s\nMemory: %s / %s\nNet I/O: %s / %s\nBlock I/O: %s / %s\nPIDs: %d\n",
		formatCPUPercent(stats),
		formatBytes(stats.MemoryStats.Usage), formatBytes(stats.MemoryStats.Limit),
		formatBytes(rxBytes), formatBytes(txBytes),
		formatBytes(readBytes), formatBytes(writeBytes),
		stats.PidsStats.Current,
	)
}
