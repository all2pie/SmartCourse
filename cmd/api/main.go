package main

import (
	"fmt"
	"log"

	"github.com/all2pie/go-smart-course/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	fmt.Printf("SmartCourse API starting — env=%s\n", cfg.AppEnv)
}
