package main

import (
	"flag"
	"log"

	"github.com/Gabriel-Valin/dockzy/internal/app"
)

func main() {
	all := flag.Bool("all", false, "carrega todos os containers/imagens/volumes do host, mesmo dentro de um projeto docker-compose")
	flag.Parse()

	if err := app.Run(*all); err != nil {
		log.Fatal(err)
	}
}
