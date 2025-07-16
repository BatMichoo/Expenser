package handlers

import (
	database "expenser/internal/db"
	"expenser/internal/middleware"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(router *gin.Engine, db *database.DB, authService *middleware.AuthService) {
	// Public routes (no authentication required)
	rootHandler := NewRootHandler(db)
	router.GET("/", rootHandler.GetRoot)

	// JWT Authentication Demo (separate from main app)
	authDemoHandler := NewAuthDemoHandler()
	router.GET("/auth-demo", authDemoHandler.GetAuthDemo)
	router.GET("/auth-demo/info", authDemoHandler.GetAuthDemoInfo)

	// JWT API routes for testing authentication backend
	if authService != nil {
		apiHandler := NewAPIHandler(db, authService)
		apiGroup := router.Group("/api")
		{
			// Public API routes
			apiGroup.POST("/register", apiHandler.APIRegister)
			apiGroup.POST("/login", apiHandler.APILogin)

			// Protected API routes
			protected := apiGroup.Group("/")
			protected.Use(authService.AuthMiddleware())
			{
				protected.GET("/profile", apiHandler.APIProfile)
				protected.GET("/protected", apiHandler.APIProtectedExample)
			}
		}
	}

	// Expense routes (currently public, but JWT backend is available)
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
