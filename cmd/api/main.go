package main

import (
	"fmt"
	"log"

	"github.com/all2pie/go-smart-course/internal/config"
	"github.com/all2pie/go-smart-course/internal/server"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	fmt.Printf("SmartCourse API starting — env=%s port=%d\n", cfg.AppEnv, cfg.Port)

	if err := server.Run(cfg); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
