package handlers

import (
	"expenser/internal/models"
	"expenser/internal/services"
	"expenser/internal/utilities"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// SettingsHandler serves the reminder settings (the home server's auth
// service has the account itself: name, password).
type SettingsHandler struct {
	users SettingsStore
	// Mailer sends the dev-mode test email; nil when SMTP isn't configured.
	Mailer  services.Mailer
	Links   *services.ReminderLinkBuilder // nil: emails have no links
	DevMode bool
}

// remindersForm is what the Reminders panel saves.
type remindersForm struct {
	Email    string
	Defaults models.ReminderDefaults
}

// parseRemindersForm reads and validates the Reminders panel. The email is
// optional (empty clears it).
func parseRemindersForm(v url.Values) (*remindersForm, error) {
	email := strings.TrimSpace(v.Get("email"))
	if email != "" && !models.ValidEmail(email) {
		return nil, models.ErrInvalidEmail
	}
	defaults, err := parseReminderFields(v)
	if err != nil {
		return nil, err
	}
	if err := defaults.Validate(); err != nil {
		return nil, err
	}
	return &remindersForm{Email: email, Defaults: defaults}, nil
}

// remindersErrors are the Reminders panel's validation messages.
var remindersErrors = []errorCase{
	badInput("error.invalid_email", models.ErrInvalidEmail),
	badInput("error.invalid_reminder_defaults", models.ErrInvalidNotify, models.ErrInvalidRemindDays,
		models.ErrInvalidRepeat, models.ErrInvalidMaxReminders),
}

func remindersError(err error) error { return mapError(err, remindersErrors) }

// GetReminders opens the Reminders panel: the reminder email and the
// reminder defaults for new bills.
func (h *SettingsHandler) GetReminders(c *gin.Context) error {
	user, err := h.users.GetUserByID(c.Request.Context(), currentUserID(c))
	if err != nil {
		return err
	}
	c.HTML(http.StatusOK, "reminders-form", gin.H{"User": user, "Lang": currentLang(c), "DevMode": h.DevMode})
	return nil
}

// SaveReminders stores the Reminders panel. Defaults only affect bills
// added from now on.
func (h *SettingsHandler) SaveReminders(c *gin.Context) error {
	if err := c.Request.ParseForm(); err != nil {
		return badRequest("error.invalid_form", err)
	}
	r, err := parseRemindersForm(c.Request.PostForm)
	if err != nil {
		return remindersError(err)
	}
	if err := h.users.UpdateReminderSettings(c.Request.Context(), currentUserID(c), r.Email, r.Defaults); err != nil {
		return err
	}
	billsChanged(c, false) // the "no email" hints on the Bills page may change
	renderSuccess(c, http.StatusOK, utilities.T(currentLang(c), "settings.reminders_saved"), "")
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
	user, err := h.users.GetUserByID(c.Request.Context(), currentUserID(c))
	if err != nil {
		return err
	}
	if err := h.Mailer.Send(c.Request.Context(), services.BuildTestReminderEmail(user.Username, user.DisplayName, lang, email, time.Now(), h.Links)); err != nil {
		log.Printf("test email to %s: %v", email, err)
		renderError(c, http.StatusBadGateway, utilities.T(lang, "settings.test_failed"), err.Error())
		return nil
	}
	renderSuccess(c, http.StatusOK, utilities.T(lang, "settings.test_sent"), email)
	return nil
}
