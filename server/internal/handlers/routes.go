package handlers

import (
	database "expenser/internal/db"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(router *gin.Engine, db *database.DB) {
	rootHandler := NewRootHandler(db)

	router.GET("/", rootHandler.GetRoot)

	homeHandler := NewHomeHandler(db)

	router.GET("/home", homeHandler.GetHome)
	router.POST("/home/expenses", homeHandler.CreateExpense)
	router.GET("/home/expenses/new", homeHandler.GetNew)
	router.GET("/home/expenses/:id", homeHandler.GetExpenseById)
	router.DELETE("/home/expenses/:id", homeHandler.Delete)

	carHandler := NewCarHandler(db)

	router.GET("/car", carHandler.GetHome)
}
