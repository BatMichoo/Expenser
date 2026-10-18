package handlers

import (
	"expenser/internal/config"
	database "expenser/internal/db"
	"expenser/internal/middleware"
	"expenser/internal/services"
	"expenser/internal/services/gemini"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(router *gin.Engine, db *database.DB, cfg *config.Config) {
	// Public routes (no authentication required)
	as := services.NewAuthService(cfg.JWT.SecretKey, cfg.JWT.TokenExpiration, cfg.LanDomain)

	router.Use(middleware.LoadClaims(as))
	router.Use(middleware.I18nMiddleware(as))

	rootHandler := NewRootHandler(db, as)
	router.NoRoute(wrap(rootHandler.NotFound))
	router.GET("/", wrap(rootHandler.GetRoot))
	router.GET("/flash", wrap(rootHandler.GetFlash))
	router.POST("/settings/language", wrap(rootHandler.ChangeLanguage))

	authHandler := NewAuthHandler(rootHandler)

	router.GET("/login", wrap(authHandler.GetLogin))
	router.POST("/login", wrap(authHandler.Login))
	router.GET("/logout", wrap(authHandler.Logout))
	router.GET("/register", wrap(authHandler.GetRegister))
	router.POST("/register", wrap(authHandler.Register))

	am := middleware.NewAuthMiddleware(as)
	chartHandler := NewChartHandler(db)
	searchHandler := NewSearchHandler(db)

	houseHandler := NewHouseHandler(rootHandler)
	protectedHouse := router.Group("/house")
	{
		protectedHouse.Use(am.AuthMiddleware())

		protectedHouse.GET("", wrap(houseHandler.GetHome))
		protectedHouse.GET("/current", wrap(houseHandler.GetCurrentMonth))
		protectedHouse.GET("/search", wrap(searchHandler.GetSearch))
		protectedHouse.POST("/search", wrap(searchHandler.GetResultsHouse))
		protectedHouse.GET("/chart", chartHandler.HouseRoot)
		protectedHouse.GET("/chart/search", chartHandler.HouseSearch)
		protectedHouse.GET("/expenses/new", wrap(houseHandler.GetCreateHouseForm))
		protectedHouse.GET("/expenses/metadata", wrap(houseHandler.GetMetadataFields))
		protectedHouse.POST("/expenses", wrap(houseHandler.CreateHouseExpense))
		protectedHouse.GET("/expenses/edit/:id", wrap(houseHandler.GetEditHouseForm))
		protectedHouse.PUT("/expenses/:id", wrap(houseHandler.EditHouseExpenseById))
		protectedHouse.GET("/expenses/delete/:id", wrap(houseHandler.GetDeleteConfirm))
		protectedHouse.DELETE("/expenses/:id", wrap(houseHandler.DeleteHouseExp))
	}

	groceriesHandler := NewGroceriesHandler(rootHandler, gemini.NewGeminiService(cfg.GeminiAPIKey))
	protectedGroceries := router.Group("/groceries")
	{
		protectedGroceries.Use(am.AuthMiddleware())
		protectedGroceries.POST("/upload", wrap(groceriesHandler.UploadReceipt))
		protectedGroceries.POST("/confirm-batch", wrap(groceriesHandler.ConfirmBatchReceipt))
		protectedGroceries.GET("", wrap(groceriesHandler.GetGroceriesHome))
		protectedGroceries.GET("/current", wrap(groceriesHandler.GetCurrentMonth))
		protectedGroceries.GET("/search", wrap(searchHandler.GetSearch))
		protectedGroceries.POST("/search", wrap(searchHandler.GetResultsGroceries))
		protectedGroceries.GET("/chart", chartHandler.GroceriesRoot)
		protectedGroceries.GET("/chart/search", chartHandler.GroceriesChartSearch)
		protectedGroceries.POST("", wrap(groceriesHandler.PostCreateGroceriesExpense))
		protectedGroceries.GET("/form", wrap(groceriesHandler.GetGroceriesForm))
		protectedGroceries.GET("/create", wrap(groceriesHandler.GetCreateGroceriesForm))
		protectedGroceries.GET("/edit/:id", wrap(groceriesHandler.GetEditGroceriesForm))
		protectedGroceries.PUT("/:id", wrap(groceriesHandler.EditGroceriesExpense))
		protectedGroceries.DELETE("/:id", wrap(groceriesHandler.DeleteGroceriesExpense))
	}
	carHandler := NewCarHandler(rootHandler)
	protectedCar := router.Group("/car")
	{
		protectedCar.Use(am.AuthMiddleware())

		protectedCar.GET("", wrap(carHandler.GetHome))
		protectedCar.GET("/current", wrap(carHandler.GetCurrentMonth))
		protectedCar.GET("/search", wrap(searchHandler.GetSearch))
		protectedCar.POST("/search", wrap(searchHandler.GetResultsCar))
		protectedCar.GET("/chart", chartHandler.CarRoot)
		protectedCar.GET("/chart/search", chartHandler.CarSearch)
		protectedCar.GET("/expenses/new", wrap(carHandler.GetCreateCarForm))
		protectedCar.GET("/expenses/metadata", wrap(carHandler.GetMetadataFields))
		protectedCar.POST("/expenses", wrap(carHandler.CreateCarExpense))
		protectedCar.GET("/expenses/edit/:id", wrap(carHandler.GetEditCarForm))
		protectedCar.PUT("/expenses/:id", wrap(carHandler.EditCarExpenseById))
		protectedCar.GET("/expenses/delete/:id", wrap(carHandler.GetDeleteConfirm))
		protectedCar.DELETE("/expenses/:id", wrap(carHandler.DeleteCarExp))
	}
}
