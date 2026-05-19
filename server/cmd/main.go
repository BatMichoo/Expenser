package main

import (
	"expenser/internal/config"
	database "expenser/internal/db"
	"expenser/internal/handlers"
	"expenser/internal/utilities"
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

	funcMap := template.FuncMap{
		"contains": func(slice interface{}, item string) bool {
			s, ok := slice.([]string)
			if !ok {
				return false
			}
			for _, val := range s {
				if val == item {
					return true
				}
			}
			return false
		},
		"T": func(lang, key string) string {
			return utilities.T(lang, key)
		},
		"dict": func(values ...interface{}) (map[string]interface{}, error) {
			if len(values)%2 != 0 {
				return nil, fmt.Errorf("invalid dict call")
			}
			dict := make(map[string]interface{}, len(values)/2)
			for i := 0; i < len(values); i += 2 {
				key, ok := values[i].(string)
				if !ok {
					return nil, fmt.Errorf("dict keys must be strings")
				}
				dict[key] = values[i+1]
			}
			return dict, nil
		},
		"emptySlice": func() []string {
			return []string{}
		},
		"float64": func(v interface{}) float64 {
			switch i := v.(type) {
			case float64:
				return i
			case int64:
				return float64(i)
			case int:
				return float64(i)
			default:
				return 0.0
			}
		},
		"div": func(a, b float64) float64 {
			if b == 0 {
				return 0
			}
			return a / b
		},
	}

	t := template.Must(template.New("").Funcs(funcMap).ParseGlob(tPath))
	router.SetHTMLTemplate(t) // Tell Gin to use this template set

	db, err := database.InitDatabase(cfg)
	if err != nil {
		log.Fatalln("Couldn't initialize database.")
	}

	if err := utilities.InitI18n(); err != nil {
		log.Fatalf("Couldn't initialize i18n: %v", err)
	}

	var sPath string
	if cfg.Mode == "" {
		sPath = filepath.Join(config.GetProjectRootDir(), "static")
	} else {
		sPath = "../static"
	}

	router.Static("/static", sPath)

	handlers.RegisterRoutes(router, db, cfg)

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
