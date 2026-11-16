package handlers

import (
	"expenser/internal/middleware"
	"expenser/internal/models"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes builds the handlers from d and wires every route.
func RegisterRoutes(router *gin.Engine, d Deps) {
	h := newHandlers(d)
	rootHandler, snoozeHandler := h.root, h.snooze
	houseHandler, carHandler, groceriesHandler := h.house, h.car, h.groceries
	sizesHandler, billsHandler := h.sizes, h.bills
	backfillHandler, chartHandler := h.backfill, h.chart

	router.Use(middleware.SameOrigin())
	router.Use(middleware.I18nMiddleware())
	router.Use(withSite(site{AuthURL: d.AuthURL, LanDomain: d.LanDomain, HomeServerURL: d.HomeServerURL}))

	// Public: the home server's nginx lets these through without a login
	// (its expenser site has a public location for /snooze/).
	router.NoRoute(wrap(rootHandler.NotFound))
	// For the home server's container health check and deployer (its nginx
	// keeps /healthz from the outside).
	router.GET("/healthz", health(d.Store))
	// Snooze links in reminder emails: signed, so no login needed.
	router.GET("/snooze/:token", wrap(snoozeHandler.GetSnooze))
	router.POST("/snooze/:token", wrap(snoozeHandler.PostSnooze))

	// Everything else is for the person the home server logged in.
	user := middleware.RequireUser(d.Store, d.DevMode)
	router.GET("/", user, wrap(rootHandler.GetRoot))
	router.GET("/flash", user, wrap(rootHandler.GetFlash))
	router.POST("/settings/language", user, wrap(rootHandler.ChangeLanguage))

	protectedHouse := router.Group("/house")
	{
		protectedHouse.Use(user)

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
		protectedGroceries.Use(user)
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
	protectedBills := router.Group("/bills")
	{
		protectedBills.Use(user)

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
		if d.DevMode {
			protectedBills.POST("/test-email", wrap(billsHandler.SendTestEmail))
		}
	}

	protectedCar := router.Group("/car")
	{
		protectedCar.Use(user)

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
