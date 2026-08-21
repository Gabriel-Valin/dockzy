package ui

import "github.com/Gabriel-Valin/dockzy/internal/docker"

type Data struct {
	StatusTitle string
	Services    []docker.Service
	Standalone  []docker.Standalone
	Images      []docker.Image
	Volumes     []docker.Volume
	Logs        string
	Stats       string
	Config      string
	Top         string
}
