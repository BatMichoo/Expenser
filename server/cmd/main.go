package main

import (
	"context"
	"expenser/internal/config"
	database "expenser/internal/db"
	"expenser/internal/handlers"
	"expenser/internal/services"
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
		log.Fatalf("Couldn't load configuration: %v", err)
	}

	var tPath string
	if cfg.Mode == "" {
		tPath = filepath.Join(config.GetProjectRootDir(), "internal/templates/**/*.html")
	} else {
		tPath = "internal/templates/**/*.html"
	}

	t := template.Must(template.New("").Funcs(utilities.TemplateFuncs()).ParseGlob(tPath))
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

	schedulerCtx, stopScheduler := context.WithCancel(context.Background())
	schedulerDone := make(chan struct{})
	reminderScheduler := services.NewReminderScheduler(db, reminderNotifier(cfg), cfg.ReminderCheckInterval)
	go func() {
		reminderScheduler.Run(schedulerCtx)
		close(schedulerDone)
	}()

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
	stopScheduler()
	<-schedulerDone
}

// reminderNotifier emails reminders when SMTP is configured; otherwise
// they are only written to the log (development, CI).
func reminderNotifier(cfg *config.Config) services.Notifier {
	smtpCfg := services.SMTPConfig{
		Host:     cfg.SMTP.Host,
		Port:     cfg.SMTP.Port,
		User:     cfg.SMTP.User,
		Password: cfg.SMTP.Password,
		From:     cfg.SMTP.From,
	}
	if !smtpCfg.Configured() {
		log.Println("SMTP not configured: reminders are logged, not emailed.")
		return services.NewConsoleNotifier(nil)
	}
	log.Printf("Reminders are emailed via %s:%s.", smtpCfg.Host, smtpCfg.Port)
	return services.NewEmailNotifier(services.NewSMTPMailer(smtpCfg))
}
