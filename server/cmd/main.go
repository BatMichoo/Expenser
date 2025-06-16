package main

import (
	"Expenser/Backend/internal/router"
	"log"
	"os"
)

func main() {
	router := router.CreateRouter()

	// --- Start the server ---
	port := os.Getenv("APP_PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Server starting on port %s", port)
	if err := router.Run(":" + port); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
