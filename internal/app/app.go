package app

import (
	"context"
	"os/signal"
	"syscall"

	"github.com/romanpitatelev/vk-subpub/internal/configs"
)

func Run(cfg *configs.Config) error {
	_, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	return nil
}
