package main

import (
	"context"
	"errors"
	"expenser/internal/config"
	database "expenser/internal/db"
	"expenser/internal/handlers"
	"expenser/internal/services"
	"expenser/internal/templates"
	"expenser/internal/utilities"
	"expenser/static"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	_ "time/tzdata" // the image has no zoneinfo; APP_TIMEZONE needs it

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
	// Dates and times typed in forms or read off receipts are wall-clock
	// times in the household's zone, and "today" for reminders is its day.
	time.Local = cfg.Location
	log.Printf("Time zone: %s.", cfg.Location)

	t := template.Must(utilities.ParseTemplates(templates.FS, templates.Patterns...))
	router.SetHTMLTemplate(t) // Tell Gin to use this template set

	db, err := database.InitDatabase(cfg)
	if err != nil {
		log.Fatalf("Couldn't initialize database: %v", err)
	}

	if err := utilities.InitI18n(); err != nil {
		log.Fatalf("Couldn't initialize i18n: %v", err)
	}

	// Asset URLs carry a version that changes on every deploy (see the asset
	// template function), so browsers may keep them for a while.
	assets := router.Group("/static", func(c *gin.Context) {
		c.Header("Cache-Control", "public, max-age=86400")
	})
	assets.StaticFS("/", http.FS(static.FS))
	// Browsers ask for /favicon.ico on pages without an icon link; without
	// this the 404 pops up as an error on the next page.
	router.StaticFileFS("/favicon.ico", "favicon.png", http.FS(static.FS))

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

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		log.Printf("Server starting on port %s", port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	// Let requests in flight finish (a deploy restarts the container), within
	// Docker's 10 second stop timeout.
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("Server shutdown: %v", err)
	}
	stopScheduler()
	<-schedulerDone
	if err := db.Close(); err != nil {
		log.Printf("Closing database: %v", err)
	}
}

// shutdownTimeout is how long requests in flight get to finish on shutdown.
const shutdownTimeout = 8 * time.Second

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
