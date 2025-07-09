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
	router.GET("/home/expenses/new", homeHandler.GetCreateForm)
	router.POST("/home/expenses", homeHandler.CreateExpense)
	router.GET("/home/expenses/edit", homeHandler.GetEditForm)
	router.GET("/home/expenses/:id", homeHandler.GetExpenseById)
	router.PUT("/home/expenses/:id", homeHandler.EditExpenseById)
	router.DELETE("/home/expenses/:id", homeHandler.Delete)

	carHandler := NewCarHandler(db)

	router.GET("/car", carHandler.GetHome)
}
