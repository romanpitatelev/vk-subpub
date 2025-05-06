package app

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"

	"github.com/romanpitatelev/vk-subpub/internal/configs"
	"github.com/romanpitatelev/vk-subpub/internal/server"
)

func Run(cfg *configs.Config) error {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	grpcServer := server.New(
		server.Config{
			Port:          fmt.Sprintf(":%d", cfg.AppPort),
			Timeout:       cfg.Timeout,
			MessageBuffer: cfg.MessageBuffer,
		})

	if err := grpcServer.Run(ctx); err != nil {
		return fmt.Errorf("failed to run subscription grpc server: %w", err)
	}

	return nil
}
