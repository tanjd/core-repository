package main

import (
	"fmt"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/danielgtaylor/huma/v2/humacli"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/rs/zerolog/pkgerrors"
	"github.com/tanjd/core-repository/apps/identity/api"
	"github.com/tanjd/core-repository/apps/identity/config"
	"github.com/tanjd/core-repository/apps/identity/handler"
	"github.com/tanjd/core-repository/apps/identity/repo"

	_ "github.com/danielgtaylor/huma/v2/formats/cbor"
)

type Options struct {
	Port int `help:"Port to listen on" short:"p" default:"8888"`
}

func main() {
	cli := humacli.New(func(hooks humacli.Hooks, options *Options) {

		r := repo.NewInMemoryRepo()
		userHandler := handler.NewUserHandler(r)
		router := chi.NewMux()
		router.Use(middleware.Logger)

		routes := api.NewRouter(userHandler, humachi.New(router, huma.DefaultConfig("Identity", "1.0.0")))
		routes.AddHealthCheckRoutes()
		routes.AddUserRoutes()

		config := config.LoadConfig()
		configureLogger(config)

		hooks.OnStart(func() {
			log.Info().
				Interface("config", config).
				Msgf("Starting server on port %d...", options.Port)

			err := http.ListenAndServe(fmt.Sprintf(":%d", options.Port), router)
			if err != nil {
				log.Fatal().Err(err).Msg("Router failed to start")
			}
		})
	})

	cli.Run()
}

func configureLogger(c config.Config) {
	if !c.LogJSON {
		log.Logger = log.Logger.Output(zerolog.NewConsoleWriter())
	}

	level, err := zerolog.ParseLevel(c.LogLevel)
	if err != nil {
		log.Fatal().Err(err).Msg("Invalid log level configuration")
	}
	log.Logger = log.Logger.Level(level)
	zerolog.DefaultContextLogger = &log.Logger
	zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack
}
