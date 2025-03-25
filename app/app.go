package app

import (
	"cnaasprom/config"
	"cnaasprom/metrics"
	"fmt"
	"log"
	"net/http"
)

type App struct {
	Config *config.Config
}

func NewApp(cfg *config.Config) *App {
	return &App{Config: cfg}
}

func (a *App) Run() error {
	address := fmt.Sprintf("%s:%d", a.Config.Server.Address, a.Config.Server.Port)

	// Set up the metrics handler
	handler := metrics.MetricsHandler(a.Config.MetricsStatisticsCategory, a.Config.QueryParams, a.Config.RemoteStatisticServer.Address, a.Config.RemoteStatisticServer.Port)

	http.Handle("/metrics", handler)

	log.Printf("Serving metrics on %s", address)
	return http.ListenAndServe(address, nil)
}
