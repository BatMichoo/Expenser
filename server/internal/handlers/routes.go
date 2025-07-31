package handlers

import (
	"expenser/internal/config"
	database "expenser/internal/db"
	"expenser/internal/middleware"
	"expenser/internal/services"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(router *gin.Engine, db *database.DB, cfg *config.Config) {
	// Public routes (no authentication required)
	as := services.NewAuthService(cfg.JWT.SecretKey, cfg.JWT.TokenExpiration)

	rootHandler := NewRootHandler(db, as)
	router.GET("/", rootHandler.GetRoot)

	authHandler := NewAuthHandler(db, as)

	router.GET("/login", authHandler.GetLogin)
	router.POST("/login", authHandler.Login)
	router.GET("/logout", authHandler.Logout)
	router.GET("/register", authHandler.GetRegister)
	router.POST("/register", authHandler.Register)

	am := middleware.NewAuthMiddleware(as)

	homeHandler := NewHomeHandler(db)
	protectedHome := router.Group("/home")
	{
		protectedHome.Use(am.AuthMiddleware())

		protectedHome.GET("", homeHandler.GetHome)
		protectedHome.GET("/expenses/new", homeHandler.GetCreateHomeForm)
		protectedHome.POST("/expenses", homeHandler.CreateHomeExpense)
		protectedHome.GET("/expenses/edit", homeHandler.GetEditHomeForm)
		protectedHome.PUT("/expenses/:id", homeHandler.EditHomeExpenseById)
		protectedHome.DELETE("/expenses/:id", homeHandler.DeleteHomeExp)
	}

	carHandler := NewCarHandler(db)
	protectedCar := router.Group("/car")
	{
		protectedCar.Use(am.AuthMiddleware())

		protectedCar.GET("", carHandler.GetHome)
		protectedCar.GET("/expenses/new", carHandler.GetCreateCarForm)
		protectedCar.POST("/expenses", carHandler.CreateCarExpense)
		protectedCar.GET("/expenses/edit", carHandler.GetEditCarForm)
		protectedCar.PUT("/expenses/:id", carHandler.EditCarExpenseById)
		protectedCar.DELETE("/expenses/:id", carHandler.DeleteCarExp)
	}
}
