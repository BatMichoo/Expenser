package handlers

import (
	"expenser/internal/models"
	"expenser/internal/utilities"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

func RenderErrorModal(c *gin.Context, err error, httpCode int) {
	lang := c.GetString("lang")
	content := &models.ModalContent{
		Title:   utilities.T(lang, "modal.error_title"),
		Message: fmt.Sprintf("%v", err),
		Lang:    lang,
	}
	c.HTML(httpCode, utilities.Templates.Components.ModalError, content)
}

func createHtmlResponseFunction(c *gin.Context, err error, isLoggedIn bool, partialTemplate string, pageData any) func() {
	lang := c.GetString("lang")

	if err != nil {
		return func() {
			httpCodeError := http.StatusInternalServerError

			content := &models.ModalContent{
				Title:   utilities.T(lang, "modal.error_title"),
				Message: fmt.Sprintf("%v: %v", httpCodeError, err),
				Lang:    lang,
			}
			c.HTML(httpCodeError, utilities.Templates.Components.ModalError, content)
		}
	}

	if c.GetHeader("HX-Request") == "true" {
		return func() {
			c.HTML(http.StatusOK, partialTemplate, pageData)
		}
	}

	rl := &models.RootLayout{
		TemplateName:    utilities.Templates.Pages.Car,
		TemplateContent: pageData,
		HeaderOpts: &models.HeaderOptions{
			IsLoggedIn: isLoggedIn,
			Lang:       lang,
		},
		Lang: lang,
	}

	return func() {
		c.HTML(http.StatusOK, utilities.Templates.Root, rl)
	}
}

func RenderPage(c *gin.Context, isLoggedIn bool, partialTemplate string, pageData any) {
	lang := c.GetString("lang")

	if c.GetHeader("HX-Request") == "true" {
		c.HTML(http.StatusOK, partialTemplate, pageData)
		return
	}

	year := time.Now().Year()

	rl := &models.RootLayout{
		TemplateName:    partialTemplate,
		TemplateContent: pageData,
		HeaderOpts: &models.HeaderOptions{
			IsLoggedIn: isLoggedIn,
			Lang:       lang,
		},
		Lang: lang,
		Year: year,
	}
	c.HTML(http.StatusOK, utilities.Templates.Root, rl)
}
