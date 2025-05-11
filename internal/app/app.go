package app

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"

	"github.com/romanpitatelev/vk-subpub/internal/configs"
	"github.com/romanpitatelev/vk-subpub/internal/server"
	"github.com/romanpitatelev/vk-subpub/internal/subpub"
)

func Run(cfg *configs.Config) error {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	subPub := subpub.NewSubPub(cfg.MessageBuffer)

	grpcServer := server.New(
		server.Config{
			Port:          cfg.BindAddress,
			Timeout:       cfg.Timeout,
			MessageBuffer: cfg.MessageBuffer,
		}, subPub)

	if err := grpcServer.Run(ctx); err != nil {
		return fmt.Errorf("failed to run subscription grpc server: %w", err)
	}

	return nil
}
