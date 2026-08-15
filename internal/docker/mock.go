package docker

// Image = imagem local. -> cli.ImageList
type Image struct {
	Repo string // "bumper", "<none>"
	Tag  string // "latest", "<none>"
	Size string // coluna da direita (aqui um contador qualquer)
	ID   string
}

// Volume = volume local. -> cli.VolumeList
type Volume struct {
	Driver string // "local"
	Name   string // hash/nome do volume
}

// MockImages devolve imagens fixas até a listagem real (cli.ImageList) ser
// implementada.
func MockImages() []Image {
	return []Image{
		{Repo: "<none>", Tag: "<none>", Size: "1", ID: "img001"},
		{Repo: "<none>", Tag: "<none>", Size: "1", ID: "img002"},
		{Repo: "bumper", Tag: "latest", Size: "9", ID: "img003"},
		{Repo: "<none>", Tag: "<none>", Size: "5", ID: "img004"},
	}
}

// MockVolumes devolve volumes fixos até a listagem real (cli.VolumeList)
// ser implementada.
func MockVolumes() []Volume {
	return []Volume{
		{Driver: "local", Name: "68473898e49e0e4d2c5998f68c79"},
		{Driver: "local", Name: "90d4357f8a93810ee8f2a59671e7"},
		{Driver: "local", Name: "b9a4a32a2d4d6366f9a9434aff8a"},
		{Driver: "local", Name: "bc80a55695b8d780db6fbe54f31a"},
	}
}
