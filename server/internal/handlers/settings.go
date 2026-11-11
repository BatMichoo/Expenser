package handlers

import (
	"expenser/internal/models"
	"expenser/internal/utilities"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// SettingsHandler serves per-user settings that need a logged-in user.
type SettingsHandler struct {
	BaseHandler BaseHandler
}

func NewSettingsHandler(rh *RootHandler) *SettingsHandler {
	return &SettingsHandler{BaseHandler: rh.BaseHandler}
}

// GetReminderEmailForm opens the "Reminder email" panel.
func (h *SettingsHandler) GetReminderEmailForm(c *gin.Context) error {
	user, err := h.BaseHandler.DB.GetUserByID(currentUserID(c))
	if err != nil {
		return err
	}
	c.HTML(http.StatusOK, "reminder-email-form", gin.H{"Email": user.Email, "Lang": currentLang(c)})
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
