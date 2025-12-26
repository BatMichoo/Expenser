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
	router.NoRoute(rootHandler.NotFound)
	router.GET("/", rootHandler.GetRoot)

	authHandler := NewAuthHandler(db, as)

	router.GET("/login", authHandler.GetLogin)
	router.POST("/login", authHandler.Login)
	router.GET("/logout", authHandler.Logout)
	router.GET("/register", authHandler.GetRegister)
	router.POST("/register", authHandler.Register)

	am := middleware.NewAuthMiddleware(as)
	chartHandler := NewChartHandler(db)
	searchHandler := NewSearchHandler(db)

	houseHandler := NewHouseHandler(db)
	protectedHouse := router.Group("/house")
	{
		protectedHouse.Use(am.AuthMiddleware())

		protectedHouse.GET("", houseHandler.GetHome)
		protectedHouse.GET("/current", houseHandler.GetCurrentMonth)
		protectedHouse.GET("/search", searchHandler.GetSearch)
		protectedHouse.POST("/search", searchHandler.GetResultsHouse)
		protectedHouse.GET("/chart", chartHandler.HouseRoot)
		protectedHouse.GET("/chart/search", chartHandler.HouseSearch)
		protectedHouse.GET("/expenses/new", houseHandler.GetCreateHouseForm)
		protectedHouse.POST("/expenses", houseHandler.CreateHouseExpense)
		protectedHouse.GET("/expenses/edit/:id", houseHandler.GetEditHouseForm)
		protectedHouse.PUT("/expenses/:id", houseHandler.EditHouseExpenseById)
		protectedHouse.GET("/expenses/delete/:id", houseHandler.GetDeleteConfirm)
		protectedHouse.DELETE("/expenses/:id", houseHandler.DeleteHouseExp)
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
		protectedCar.GET("/chart", chartHandler.CarRoot)
		protectedCar.GET("/chart/search", chartHandler.CarSearch)
		// protectedCar.GET("/expenses/search", searchHandler.GetSearch)
	}

}
