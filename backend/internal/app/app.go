package app

import (
	"net/http"

	"github.com/rs/zerolog/log"

	"github.com/zurco34/pelican-mc-router/internal/api"
)

func Run() error {

	router := api.NewRouter()

	log.Info().
		Str("address", ":8080").
		Msg("Starting HTTP server")

	return http.ListenAndServe(":8080", router)
}