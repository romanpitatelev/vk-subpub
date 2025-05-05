package main

import (
	"github.com/romanpitatelev/vk-subpub/internal/app"
	"github.com/romanpitatelev/vk-subpub/internal/configs"
)

func main() {
	cfg := configs.New()

	if err := app.Run(cfg); err != nil {
		panic(err)
	}
}
