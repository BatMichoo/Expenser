package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func CreateRouter() *gin.Engine {

	router := gin.Default()

	router.LoadHTMLGlob("internal/templates/**/*.html") // Load all templates

	router.Static("/static", "./static") // Serve files from ./static directory under /static URL path

	router.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "base.html", gin.H{
			"Title":   "My HTMX App",
			"Message": "Welcome!",
		})
	})

	return router
}
