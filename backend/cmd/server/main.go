package main

import (
	"log"
	"os"

	"kazokucal/backend/internal/server"
)

func main() {
	cfg := server.ConfigFromEnv()
	router := server.NewRouter(cfg)

	log.Printf("starting KazokuCal on :%s", cfg.Port)
	if err := router.Run(":" + cfg.Port); err != nil {
		log.Fatal(err)
	}
}

func init() {
	log.SetOutput(os.Stdout)
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}
