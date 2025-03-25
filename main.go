package main

import (
	"cnaasprom/app"
	"cnaasprom/config"
	"log"
)

func main() {
	// Load configuration
	loadedConfig, err := config.LoadConfig("config.yaml")
	if err != nil {
		log.Fatalf("Error loading configuration: %v", err)
	}

	// Initialize and run the application
	application := app.NewApp(loadedConfig)
	if err := application.Run(); err != nil {
		log.Fatalf("Application failed: %v", err)
	}
}
