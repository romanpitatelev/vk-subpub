package configs

import (
	"fmt"
	"os"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
	"github.com/rs/zerolog/log"
)

type Config struct {
	BindAddress   string        `env:"BIND_ADDRESS" env-default:":9000" env-description:"Application port"`
	Timeout       time.Duration `env:"TIMEOUT" env-default:"5s" env-description:"Timeout"`
	MessageBuffer int           `env:"MESSAGE_BUFFER" env-default:"100" emv-description:"Message buffer"`
}

func (c *Config) getHelpString() (string, error) {
	baseHeader := "Environment variables that can be set with env: "

	helpString, err := cleanenv.GetDescription(c, &baseHeader)
	if err != nil {
		return "", fmt.Errorf("failed to get help string: %w", err)
	}

	return helpString, nil
}

func New() *Config {
	cfg := &Config{}

	helpString, err := cfg.getHelpString()
	if err != nil {
		log.Panic().Err(err).Msg("failed to get help string")
	}

	log.Info().Msg(helpString)

	if err := cleanenv.ReadEnv(cfg); err != nil {
		log.Panic().Err(err).Msg("failed to read config from envs")
	}

	if err = cleanenv.ReadConfig(".env", cfg); err != nil && !os.IsNotExist(err) {
		log.Panic().Err(err).Msg("failed to read config from .env")
	}

	return cfg
}
