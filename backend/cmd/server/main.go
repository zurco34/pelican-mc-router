package main

import (
	"log"

	"github.com/zurco34/pelican-mc-router/internal/app"
)

func main() {
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}