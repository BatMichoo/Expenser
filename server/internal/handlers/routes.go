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
	router.GET("/home/expenses/new", homeHandler.GetCreateHomeForm)
	router.POST("/home/expenses", homeHandler.CreateHomeExpense)
	router.GET("/home/expenses/edit", homeHandler.GetEditHomeForm)
	router.PUT("/home/expenses/:id", homeHandler.EditHomeExpenseById)
	router.DELETE("/home/expenses/:id", homeHandler.DeleteHomeExp)

	carHandler := NewCarHandler(db)

	router.GET("/car", carHandler.GetHome)
	router.GET("/car/expenses/new", carHandler.GetCreateCarForm)
	router.POST("/car/expenses", carHandler.CreateCarExpense)
	router.GET("/car/expenses/edit", carHandler.GetEditCarForm)
	router.PUT("/car/expenses/:id", carHandler.EditCarExpenseById)
	router.DELETE("/car/expenses/:id", carHandler.DeleteCarExp)
}
