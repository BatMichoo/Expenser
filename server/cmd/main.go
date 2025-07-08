package main

import (
	"expenser/internal/config"
	database "expenser/internal/db"
	"expenser/internal/handlers"
	"fmt"
	"html/template"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/gin-gonic/gin"
)

func main() {
	router := gin.Default()

	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalln("Couldn't load configuration.")
	}

	var tPath string
	if cfg.Mode == "" {
		tPath = filepath.Join(config.GetProjectRootDir(), "internal/templates/**/*.html")
	} else {
		tPath = "internal/templates/**/*.html"
	}

	t := template.Must(template.ParseGlob(tPath))
	router.SetHTMLTemplate(t) // Tell Gin to use this template set

	db, err := database.InitDatabase(cfg)
	if err != nil {
		log.Fatalln("Couldn't initialize database.")
	}

	router.Static("/static", "../static")

	handlers.RegisterRoutes(router, db)

	// --- Start the server ---
	port := os.Getenv("APP_PORT")
	if port == "" {
		port = "8080"
	}

	go func() {
		log.Printf("Server starting on port %s", port)
		if err := router.Run(":" + port); err != nil {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	fmt.Println("Shutting down server...")
}
