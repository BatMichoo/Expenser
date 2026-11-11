package handlers

import (
	"errors"
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

// SettingsHandler serves per-user settings that need a logged-in user.
type SettingsHandler struct {
	BaseHandler BaseHandler
	// Mailer sends the dev-mode test email; nil when SMTP isn't configured.
	Mailer  services.Mailer
	Links   *services.ReminderLinkBuilder // nil: emails have no links
	DevMode bool
}

func NewSettingsHandler(rh *RootHandler, mailer services.Mailer, links *services.ReminderLinkBuilder, devMode bool) *SettingsHandler {
	return &SettingsHandler{BaseHandler: rh.BaseHandler, Mailer: mailer, Links: links, DevMode: devMode}
}

// accountForm is what Settings > Account saves.
type accountForm struct {
	DisplayName string
	Email       string
	Defaults    models.ReminderDefaults
}

// parseAccountForm reads and validates the Account panel. The name and
// email are optional (empty clears them).
func parseAccountForm(v url.Values) (*accountForm, error) {
	name, err := models.CleanDisplayName(v.Get("name"))
	if err != nil {
		return nil, err
	}
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
	return &accountForm{DisplayName: name, Email: email, Defaults: defaults}, nil
}

// accountError turns validation errors into user-facing messages.
func accountError(err error) error {
	switch {
	case errors.Is(err, models.ErrInvalidDisplayName):
		return badRequest("error.invalid_display_name", err)
	case errors.Is(err, models.ErrInvalidEmail):
		return badRequest("error.invalid_email", err)
	case errors.Is(err, models.ErrInvalidNotify),
		errors.Is(err, models.ErrInvalidRemindDays),
		errors.Is(err, models.ErrInvalidRepeat),
		errors.Is(err, models.ErrInvalidMaxReminders):
		return badRequest("error.invalid_reminder_defaults", err)
	}
	return err
}

// GetAccountForm opens the Account panel: name, reminder email and the
// reminder defaults for new bills.
func (h *SettingsHandler) GetAccountForm(c *gin.Context) error {
	user, err := h.BaseHandler.DB.GetUserByID(currentUserID(c))
	if err != nil {
		return err
	}
	c.HTML(http.StatusOK, "account-form", gin.H{"User": user, "Lang": currentLang(c), "DevMode": h.DevMode})
	return nil
}

// SaveAccount stores the Account panel. Defaults only affect bills added
// from now on.
func (h *SettingsHandler) SaveAccount(c *gin.Context) error {
	if err := c.Request.ParseForm(); err != nil {
		return badRequest("error.invalid_form", err)
	}
	a, err := parseAccountForm(c.Request.PostForm)
	if err != nil {
		return accountError(err)
	}
	if err := h.BaseHandler.DB.UpdateUserAccount(c.Request.Context(), currentUserID(c), a.DisplayName, a.Email, a.Defaults); err != nil {
		return err
	}
	billsChanged(c, false) // the "no email" hints on the Bills page may change
	renderSuccess(c, http.StatusOK, utilities.T(currentLang(c), "settings.account_saved"), "")
	return nil
}

// passwordError turns a refused password change into a user-facing message.
func passwordError(err error) error {
	switch {
	case errors.Is(err, services.ErrWrongPassword):
		return badRequest("auth.wrong_current_password", err)
	case errors.Is(err, models.ErrPasswordTooShort):
		return badRequest("auth.password_too_short", err)
	case errors.Is(err, models.ErrPasswordTooLong):
		return badRequest("auth.password_too_long", err)
	case errors.Is(err, models.ErrPasswordMismatch):
		return badRequest("auth.passwords_mismatch", err)
	case errors.Is(err, models.ErrPasswordUnchanged):
		return badRequest("auth.password_unchanged", err)
	}
	return err
}

// ChangePassword sets a new password after checking the current one. The
// user stays logged in; other logins last until their token expires.
func (h *SettingsHandler) ChangePassword(c *gin.Context) error {
	err := services.ChangePassword(c.Request.Context(), h.BaseHandler.DB, currentUserID(c),
		c.PostForm("current_password"), c.PostForm("new_password"), c.PostForm("confirm_password"))
	if err != nil {
		return passwordError(err)
	}
	renderSuccess(c, http.StatusOK, utilities.T(currentLang(c), "settings.password_changed"), "")
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
	if err := h.Mailer.Send(c.Request.Context(), services.BuildTestReminderEmail(user.Username, user.DisplayName, lang, email, time.Now(), h.Links)); err != nil {
		log.Printf("test email to %s: %v", email, err)
		renderError(c, http.StatusBadGateway, utilities.T(lang, "settings.test_failed"), err.Error())
		return nil
	}
	renderSuccess(c, http.StatusOK, utilities.T(lang, "settings.test_sent"), email)
	return nil
}
