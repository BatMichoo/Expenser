package handlers

import (
	"expenser/internal/middleware"
	"expenser/internal/models"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes builds the handlers from d and wires every route.
func RegisterRoutes(router *gin.Engine, d Deps) {
	h := newHandlers(d)
	rootHandler, authHandler, snoozeHandler := h.root, h.auth, h.snooze
	houseHandler, carHandler, groceriesHandler := h.house, h.car, h.groceries
	sizesHandler, billsHandler, settingsHandler := h.sizes, h.bills, h.settings
	backfillHandler, chartHandler := h.backfill, h.chart

	router.Use(middleware.SameOrigin())
	router.Use(middleware.LoadClaims(d.Auth))
	router.Use(middleware.I18nMiddleware(d.Auth))

	// Public routes (no authentication required)
	router.NoRoute(wrap(rootHandler.NotFound))
	router.GET("/", wrap(rootHandler.GetRoot))
	router.GET("/flash", wrap(rootHandler.GetFlash))
	router.POST("/settings/language", wrap(rootHandler.ChangeLanguage))

	// Snooze links in reminder emails: signed, so no login needed.
	router.GET("/snooze/:token", wrap(snoozeHandler.GetSnooze))
	router.POST("/snooze/:token", wrap(snoozeHandler.PostSnooze))

	router.GET("/login", wrap(authHandler.GetLogin))
	router.POST("/login", wrap(authHandler.Login))
	router.GET("/logout", wrap(authHandler.Logout))
	router.GET("/register", wrap(authHandler.GetRegister))
	router.POST("/register", wrap(authHandler.Register))

	am := middleware.NewAuthMiddleware(d.Auth)

	protectedHouse := router.Group("/house")
	{
		protectedHouse.Use(am.AuthMiddleware())

		protectedHouse.GET("", wrap(houseHandler.GetHome))
		protectedHouse.GET("/current", wrap(houseHandler.GetCurrentMonth))
		protectedHouse.GET("/chart", wrap(chartHandler.Show(models.SectionHouse)))
		protectedHouse.GET("/expenses/new", wrap(houseHandler.GetCreateHouseForm))
		protectedHouse.GET("/expenses/metadata", wrap(houseHandler.GetMetadataFields))
		protectedHouse.POST("/expenses", wrap(backfillHandler.WithPreview(models.SectionHouse, houseHandler.CreateHouseExpense)))
		protectedHouse.POST("/expenses/backfill", wrap(backfillHandler.Confirm(models.SectionHouse)))
		protectedHouse.GET("/expenses/edit/:id", wrap(houseHandler.GetEditHouseForm))
		protectedHouse.PUT("/expenses/:id", wrap(houseHandler.EditHouseExpenseById))
		protectedHouse.GET("/expenses/delete/:id", wrap(houseHandler.GetDeleteConfirm))
		protectedHouse.DELETE("/expenses/:id", wrap(houseHandler.DeleteHouseExp))
	}

	protectedGroceries := router.Group("/groceries")
	{
		protectedGroceries.Use(am.AuthMiddleware())
		protectedGroceries.POST("/upload", wrap(groceriesHandler.UploadReceipt))
		protectedGroceries.GET("/upload/:id", wrap(groceriesHandler.GetReceiptJob))
		protectedGroceries.POST("/confirm-batch", wrap(groceriesHandler.ConfirmBatchReceipt))
		protectedGroceries.GET("", wrap(groceriesHandler.GetGroceriesHome))
		protectedGroceries.GET("/current", wrap(groceriesHandler.GetCurrentMonth))
		protectedGroceries.GET("/chart", wrap(chartHandler.Show(models.SectionGroceries)))
		protectedGroceries.POST("", wrap(groceriesHandler.PostCreateGroceriesExpense))
		protectedGroceries.GET("/sizes", wrap(sizesHandler.GetSizes))
		protectedGroceries.POST("/sizes/suggest", wrap(sizesHandler.Suggest))
		protectedGroceries.POST("/sizes/:id", wrap(sizesHandler.SaveSize))
		protectedGroceries.POST("/sizes/:id/skip", wrap(sizesHandler.SkipSize))
		protectedGroceries.GET("/size-guess", wrap(groceriesHandler.GetSizeGuess))
		protectedGroceries.GET("/form", wrap(groceriesHandler.GetGroceriesForm))
		protectedGroceries.GET("/create", wrap(groceriesHandler.GetCreateGroceriesForm))
		protectedGroceries.GET("/edit/:id", wrap(groceriesHandler.GetEditGroceriesForm))
		protectedGroceries.PUT("/:id", wrap(groceriesHandler.EditGroceriesExpense))
		protectedGroceries.DELETE("/:id", wrap(groceriesHandler.DeleteGroceriesExpense))
	}
	protectedSettings := router.Group("/settings")
	{
		protectedSettings.Use(am.AuthMiddleware())
		protectedSettings.GET("/account", wrap(settingsHandler.GetAccountForm))
		protectedSettings.POST("/account", wrap(settingsHandler.SaveAccount))
		protectedSettings.POST("/account/password", wrap(settingsHandler.ChangePassword))
		if d.DevMode {
			protectedSettings.POST("/account/test-email", wrap(settingsHandler.SendTestEmail))
		}
	}

	protectedBills := router.Group("/bills")
	{
		protectedBills.Use(am.AuthMiddleware())

		protectedBills.GET("", wrap(billsHandler.GetBills))
		protectedBills.GET("/current", wrap(billsHandler.GetBillsView))
		protectedBills.GET("/new", wrap(billsHandler.GetNewBillForm))
		protectedBills.GET("/types", wrap(billsHandler.GetBillTypes))
		protectedBills.POST("", wrap(billsHandler.CreateBill))
		protectedBills.GET("/edit/:id", wrap(billsHandler.GetEditBillForm))
		protectedBills.PUT("/:id", wrap(billsHandler.UpdateBill))
		protectedBills.GET("/delete/:id", wrap(billsHandler.GetDeleteBillConfirm))
		protectedBills.DELETE("/:id", wrap(billsHandler.DeleteBill))
		protectedBills.GET("/pay/:id", wrap(billsHandler.GetPayForm))
		protectedBills.POST("/:id/paid", wrap(billsHandler.MarkBillPaid))
		protectedBills.POST("/:id/skip", wrap(billsHandler.SkipMissed))
	}

	protectedCar := router.Group("/car")
	{
		protectedCar.Use(am.AuthMiddleware())

		protectedCar.GET("", wrap(carHandler.GetHome))
		protectedCar.GET("/current", wrap(carHandler.GetCurrentMonth))
		protectedCar.GET("/chart", wrap(chartHandler.Show(models.SectionCar)))
		protectedCar.GET("/expenses/new", wrap(carHandler.GetCreateCarForm))
		protectedCar.GET("/expenses/metadata", wrap(carHandler.GetMetadataFields))
		protectedCar.POST("/expenses", wrap(backfillHandler.WithPreview(models.SectionCar, carHandler.CreateCarExpense)))
		protectedCar.POST("/expenses/backfill", wrap(backfillHandler.Confirm(models.SectionCar)))
		protectedCar.GET("/expenses/edit/:id", wrap(carHandler.GetEditCarForm))
		protectedCar.PUT("/expenses/:id", wrap(carHandler.EditCarExpenseById))
		protectedCar.GET("/expenses/delete/:id", wrap(carHandler.GetDeleteConfirm))
		protectedCar.DELETE("/expenses/:id", wrap(carHandler.DeleteCarExp))
	}
}
