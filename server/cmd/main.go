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

// trustedProxies are the private networks a reverse proxy can be on.
var trustedProxies = []string{"127.0.0.1/32", "::1/128", "10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"}

func main() {
	router := gin.Default()
	// Only nginx (on the Docker network) and local tools may set
	// X-Forwarded-For; anyone else's is ignored.
	if err := router.SetTrustedProxies(trustedProxies); err != nil {
		log.Fatalf("Trusted proxies: %v", err)
	}

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

	t := template.Must(utilities.ParseTemplates(tPath))
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
	// Browsers ask for /favicon.ico on pages without an icon link; without
	// this the 404 pops up as an error on the next page.
	router.StaticFile("/favicon.ico", filepath.Join(sPath, "favicon.png"))

	mailer := smtpMailer(cfg)
	if cfg.DevMode {
		log.Println("Dev mode: developer helpers (test email) are on.")
	}
	links := services.NewReminderLinks(cfg.AppURL, services.NewSnoozeSigner(cfg.JWT.SecretKey), nil)
	switch {
	case links != nil:
		log.Printf("Reminder emails link to %s.", cfg.AppURL)
	case cfg.AppURL != "":
		log.Printf("APP_URL %q is not an http(s) address: reminder emails have no links.", cfg.AppURL)
	}
	handlers.RegisterRoutes(router, db, cfg, mailer, links)

	schedulerCtx, stopScheduler := context.WithCancel(context.Background())
	schedulerDone := make(chan struct{})
	reminderScheduler := services.NewReminderScheduler(db, reminderNotifier(mailer, links), cfg.ReminderCheckInterval)
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

// smtpMailer is the mail sender, or nil when SMTP isn't configured.
func smtpMailer(cfg *config.Config) services.Mailer {
	smtpCfg := services.SMTPConfig{
		Host:     cfg.SMTP.Host,
		Port:     cfg.SMTP.Port,
		User:     cfg.SMTP.User,
		Password: cfg.SMTP.Password,
		From:     cfg.SMTP.From,
	}
	if !smtpCfg.Configured() {
		return nil
	}
	log.Printf("Email is sent via %s:%s.", smtpCfg.Host, smtpCfg.Port)
	return services.NewSMTPMailer(smtpCfg)
}

// reminderNotifier emails reminders when there is a mailer; otherwise they
// are only written to the log (development, CI).
func reminderNotifier(mailer services.Mailer, links *services.ReminderLinkBuilder) services.Notifier {
	if mailer == nil {
		log.Println("SMTP not configured: reminders are logged, not emailed.")
		return services.NewConsoleNotifier(nil)
	}
	return services.NewEmailNotifier(mailer, links)
}
