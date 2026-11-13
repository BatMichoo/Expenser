package handlers

import (
	"context"

	"expenser/internal/models"
	"expenser/internal/services"
	"expenser/internal/services/gemini"

	"github.com/google/uuid"
)

// Each handler names the data access it needs, as SnoozeStore and
// SizesStore do, so handlers depend on behaviour rather than on the
// database package, and tests can pass a fake.

// UserStore is what the home page and the language switch need.
type UserStore interface {
	GetUserByID(ctx context.Context, id uuid.UUID) (*models.User, error)
	UpdateUserLanguage(ctx context.Context, userID uuid.UUID, lang string) error
}

// HouseStore is what the house expense forms need.
type HouseStore interface {
	GetHouseUtilityTypes(ctx context.Context) ([]models.HomeUtilityType, error)
	GetHouseExpenseByID(ctx context.Context, id int, userID uuid.UUID) (*models.HouseExpense, error)
	CreateHouseExpense(ctx context.Context, input *models.HouseExpense) error
	EditHouseExpense(ctx context.Context, e *models.HouseExpense, userID uuid.UUID) error
	DeleteHouseExpense(ctx context.Context, id int, userID uuid.UUID) (bool, error)
}

// CarStore is what the car expense forms need.
type CarStore interface {
	GetCarExpenseTypes(ctx context.Context) ([]models.CarExpenseType, error)
	GetCarExpenseByID(ctx context.Context, id int, userID uuid.UUID) (*models.CarExpense, error)
	CreateCarExpense(ctx context.Context, input *models.CarExpense) error
	EditCarExpense(ctx context.Context, e *models.CarExpense, userID uuid.UUID) error
	DeleteCarExpense(ctx context.Context, id int, userID uuid.UUID) (bool, error)
}

// GroceriesStore is what the groceries forms and receipts need.
type GroceriesStore interface {
	GetAllCategories(ctx context.Context) ([]models.Category, error)
	GetGroceriesExpenseByID(ctx context.Context, id int, userID uuid.UUID) (*models.GroceriesExpense, error)
	CreateGroceriesExpense(ctx context.Context, input *models.GroceriesExpense) error
	CreateGroceriesExpenses(ctx context.Context, items []*models.GroceriesExpense) error
	EditGroceriesExpense(ctx context.Context, input *models.GroceriesExpense, userID uuid.UUID) error
	DeleteGroceriesExpense(ctx context.Context, id int, userID uuid.UUID) (bool, error)
}

// SectionTypesStore lists a section's expense types (bill and backfill
// forms).
type SectionTypesStore interface {
	GetSectionTypes(ctx context.Context, s models.Section) ([]models.TypeOption, error)
}

// BillsStore is what the bill forms read besides the bills themselves.
type BillsStore interface {
	SectionTypesStore
	GetUserByID(ctx context.Context, id uuid.UUID) (*models.User, error)
}

// SettingsStore is what the account settings need.
type SettingsStore interface {
	GetUserByID(ctx context.Context, id uuid.UUID) (*models.User, error)
	UpdateUserAccount(ctx context.Context, id uuid.UUID, displayName, email string, d models.ReminderDefaults) error
}

// Store is all the data access of the handlers and of the services they
// use. *database.DB implements it.
type Store interface {
	UserStore
	HouseStore
	CarStore
	GroceriesStore
	BillsStore
	SettingsStore
	SnoozeStore
	SizesStore
	services.AccountStore
	services.MonthStore
	services.HomeStore
	services.TrendsStore
	services.RecurringPaymentStore
	services.BackfillStore
}

// Deps are what the routes are built from; main makes them once.
type Deps struct {
	Store     Store
	Auth      *services.AuthService
	ReceiptAI *gemini.GeminiService
	Snooze    *services.SnoozeSigner
	Mailer    services.Mailer               // nil: SMTP isn't configured
	Links     *services.ReminderLinkBuilder // nil: emails have no links
	DevMode   bool                          // developer helpers (test email)
}

// handlerSet is every handler, each built once with the services it uses.
type handlerSet struct {
	root      *RootHandler
	auth      *AuthHandler
	snooze    *SnoozeHandler
	house     *HouseHandler
	car       *CarHandler
	groceries *GroceriesHandler
	sizes     *SizesHandler
	bills     *BillsHandler
	settings  *SettingsHandler
	backfill  *BackfillHandler
	chart     *ChartHandler
}

// newHandlers builds the services once and hands them to the handlers that
// share them (the month views serve house, car and groceries).
func newHandlers(d Deps) *handlerSet {
	months := services.NewMonthViewService(d.Store)
	accounts := services.NewAccountService(d.Store)
	return &handlerSet{
		root:      &RootHandler{users: d.Store, homes: services.NewHomeService(d.Store), auth: d.Auth},
		auth:      &AuthHandler{accounts: accounts, auth: d.Auth},
		snooze:    NewSnoozeHandler(d.Store, d.Snooze),
		house:     &HouseHandler{store: d.Store, months: months},
		car:       &CarHandler{store: d.Store, months: months},
		groceries: &GroceriesHandler{store: d.Store, months: months, receipts: services.NewReceiptJobs(d.ReceiptAI.AnalyzeReceipt)},
		sizes:     NewSizesHandler(d.Store, d.ReceiptAI),
		bills:     &BillsHandler{store: d.Store, bills: services.NewRecurringPaymentService(d.Store)},
		settings: &SettingsHandler{users: d.Store, accounts: accounts, auth: d.Auth,
			Mailer: d.Mailer, Links: d.Links, DevMode: d.DevMode},
		backfill: &BackfillHandler{types: d.Store, svc: services.NewBackfillService(d.Store)},
		chart:    &ChartHandler{trends: services.NewTrendsService(d.Store)},
	}
}
