package config

import (
	"strings"
	"time"

	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/structs"
	"github.com/knadh/koanf/v2"
	"github.com/rs/zerolog/log"
)

const envVarPrefix = ""

// Config add `json:"-"` for sensitive env variable
type Config struct {
	Env                string        `config:"env"`
	LogJSON            bool          `config:"log_json"`
	LogLevel           string        `config:"log_level"`
	ServerPort         string        `config:"server_port"`
	TerminationTimeout time.Duration `config:"termination_timeout"`
	Secret             string        `json:"-" config:"secret"`
}

func DefaultConfig() Config {
	return Config{
		Env:                "production",
		ServerPort:         "8080",
		LogJSON:            false,
		LogLevel:           "error",
		TerminationTimeout: 2 * time.Second,
	}
}

var k = koanf.New(".")
var tag = "config"

func LoadConfig() Config {
	err := k.Load(structs.Provider(DefaultConfig(), tag), nil)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load config defaults")
	}

	if err := k.Load(env.Provider(envVarPrefix, ".", func(s string) string {
		return strings.ReplaceAll(strings.ToLower(
			strings.TrimPrefix(s, envVarPrefix)), "__", ".")
	}), nil); err != nil {
		log.Fatal().Err(err).Msg("Error loading environment variables")
	}

	config := Config{}
	if err := k.UnmarshalWithConf("", &config, koanf.UnmarshalConf{Tag: tag}); err != nil {
		log.Fatal().Err(err).Msg("Error unmarshal-ing config")
	}

	return config
}
