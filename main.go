package main

import (
	"log"

	"github.com/Gabriel-Valin/dockzy/internal/app"
)

func main() {
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
