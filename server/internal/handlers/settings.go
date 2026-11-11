package handlers

import (
	"expenser/internal/models"
	"expenser/internal/services"
	"expenser/internal/utilities"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// SettingsHandler serves per-user settings that need a logged-in user.
type SettingsHandler struct {
	BaseHandler BaseHandler
	// Mailer sends the dev-mode test email; nil when SMTP isn't configured.
	Mailer  services.Mailer
	DevMode bool
}

func NewSettingsHandler(rh *RootHandler, mailer services.Mailer, devMode bool) *SettingsHandler {
	return &SettingsHandler{BaseHandler: rh.BaseHandler, Mailer: mailer, DevMode: devMode}
}

// GetReminderEmailForm opens the "Reminder email" panel.
func (h *SettingsHandler) GetReminderEmailForm(c *gin.Context) error {
	user, err := h.BaseHandler.DB.GetUserByID(currentUserID(c))
	if err != nil {
		return err
	}
	c.HTML(http.StatusOK, "reminder-email-form", gin.H{"Email": user.Email, "Lang": currentLang(c), "DevMode": h.DevMode})
	return nil
}

// SaveReminderEmail stores the preferred email; empty removes it.
func (h *SettingsHandler) SaveReminderEmail(c *gin.Context) error {
	email := strings.TrimSpace(c.PostForm("email"))
	if email != "" && !models.ValidEmail(email) {
		return badRequest("error.invalid_email", models.ErrInvalidEmail)
	}
	if err := h.BaseHandler.DB.SetUserEmail(c.Request.Context(), currentUserID(c), email); err != nil {
		return err
	}
	lang := currentLang(c)
	title := utilities.T(lang, "settings.email_saved")
	if email == "" {
		title = utilities.T(lang, "settings.email_cleared")
	}
	billsChanged(c, false) // the "no email" hints on the Bills page may change
	renderSuccess(c, http.StatusOK, title, email)
	return nil
}

// SendTestEmail (dev mode only) emails a sample reminder to the address in
// the form, saved or not, so the SMTP setup can be checked right away. The
// SMTP error is shown as is: this is a developer tool.
func (h *SettingsHandler) SendTestEmail(c *gin.Context) error {
	lang := currentLang(c)
	email := strings.TrimSpace(c.PostForm("email"))
	if email == "" {
		return badRequest("settings.test_needs_email", nil)
	}
	if !models.ValidEmail(email) {
		return badRequest("error.invalid_email", models.ErrInvalidEmail)
	}
	if h.Mailer == nil {
		renderError(c, http.StatusServiceUnavailable, utilities.T(lang, "settings.test_failed"), utilities.T(lang, "settings.smtp_not_configured"))
		return nil
	}
	user, err := h.BaseHandler.DB.GetUserByID(currentUserID(c))
	if err != nil {
		return err
	}
	if err := h.Mailer.Send(c.Request.Context(), services.BuildTestReminderEmail(user.Username, lang, email, time.Now())); err != nil {
		log.Printf("test email to %s: %v", email, err)
		renderError(c, http.StatusBadGateway, utilities.T(lang, "settings.test_failed"), err.Error())
		return nil
	}
	renderSuccess(c, http.StatusOK, utilities.T(lang, "settings.test_sent"), email)
	return nil
}
