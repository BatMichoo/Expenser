package handlers

import (
	"database/sql"
	"errors"
	"expenser/internal/utilities"
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

// HandlerFunc is a gin handler that reports failures by returning an error.
// Use wrap to register it on a route.
type HandlerFunc func(c *gin.Context) error

// HTTPError is an error with a status code and a message for the user.
// MessageKey and TitleKey are i18n keys, translated when the error is
// rendered. Args fill the message's format verbs. Err holds the underlying
// cause for the log and is never shown to the user.
type HTTPError struct {
	Status     int
	TitleKey   string
	MessageKey string
	Args       []any
	Err        error
}

func (e *HTTPError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return e.MessageKey
}

func (e *HTTPError) Unwrap() error {
	return e.Err
}

// message returns the translated user message.
func (e *HTTPError) message(lang string) string {
	msg := utilities.T(lang, e.MessageKey)
	if len(e.Args) > 0 {
		msg = fmt.Sprintf(msg, e.Args...)
	}
	return msg
}

// badRequest reports invalid user input. messageKey tells the user what to fix.
func badRequest(messageKey string, err error) error {
	return &HTTPError{Status: http.StatusBadRequest, MessageKey: messageKey, Err: err}
}

// notFound reports a missing resource.
func notFound(messageKey string) error {
	return &HTTPError{Status: http.StatusNotFound, MessageKey: messageKey}
}

// errorCase turns any of Errs (matched with errors.Is) into a message for
// the user.
type errorCase struct {
	Status int
	Key    string
	Errs   []error
}

// badInput is an errorCase for invalid user input.
func badInput(key string, errs ...error) errorCase {
	return errorCase{Status: http.StatusBadRequest, Key: key, Errs: errs}
}

// mapError returns the user-facing error for the first case err matches,
// keeping err as the cause for the log; other errors pass through and
// become a generic 500.
func mapError(err error, cases []errorCase) error {
	if err == nil {
		return nil
	}
	for _, c := range cases {
		for _, target := range c.Errs {
			if errors.Is(err, target) {
				return &HTTPError{Status: c.Status, MessageKey: c.Key, Err: err}
			}
		}
	}
	return err
}

// wrap adapts a HandlerFunc to gin. A returned error is logged and rendered as
// an error modal. Any other error becomes a 500 with a generic message, so
// internal details never reach the user.
func wrap(h HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		err := h(c)
		if err == nil {
			return
		}

		lang := currentLang(c)
		status := http.StatusInternalServerError
		titleKey := "modal.error_title"
		message := utilities.T(lang, "error.message")

		var httpErr *HTTPError
		switch {
		case errors.As(err, &httpErr):
			status = httpErr.Status
			message = httpErr.message(lang)
			if httpErr.TitleKey != "" {
				titleKey = httpErr.TitleKey
			}
		case errors.Is(err, sql.ErrNoRows):
			status = http.StatusNotFound
			message = utilities.T(lang, "error.not_found")
		}

		route := c.FullPath()
		if route == "" {
			route = c.Request.URL.Path
		}
		log.Printf("%s %s: %d: %v", c.Request.Method, route, status, err)
		renderError(c, status, utilities.T(lang, titleKey), message)
	}
}
