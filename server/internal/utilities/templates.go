package utilities

// Pages defines the names for full application pages.
type Pages struct {
	Index    string // Index is the name for the main index page template.
	Register string
	Login    string
	House    string // Home is the name for the expense page template.
	Car      string
}

// HTMXComponents defines the names for reusable HTMX-specific UI components.
type HTMXComponents struct {
	Header             string // Header is the name for the application's header component.
	Footer             string // Footer is the name for the application's footer component.
	CreateHouseExpForm string // CreateExpForm is the name for the expense creation form component.
	NewHouseExp        string // NewExp is the name for the new expense component (often a row or card).
	EditHouseExpForm   string // EditExpForm is the name for the expense editing form component.
	CreateCarExpForm   string // CreateExpForm is the name for the expense creation form component.
	NewCarExp          string // NewExp is the name for the new expense component (often a row or card).
	EditCarExpForm     string // EditExpForm is the name for the expense editing form component.
	HouseExpRow        string
	CarExpRow          string
	TotalCard          string
	HighestCard        string
	Modal              string
	ModalSuccess       string
	ModalError         string
	ModalConfirm       string
	Chart              string
	HouseCurrent       string
	CarSummary         string
	Dialog             string
	Search             string
	SearchResultsHouse string
}

// Responses defines the names for specific HTMX partial responses.
// These are often fragments returned by HTMX requests that swap content on the page.
type Responses struct {
	CreateHouseExp  string // CreateHouseExp is the name for the response partial after creating a home expense.
	UpdateHouseExp  string // UpdateHomeExp is the name for the response partial after updating a home expense.
	DeleteHouseExp  string // DeleteHomeExp is the name for the response partial after deleting a home expense.
	CreateCarExp    string // CreateHomeExp is the name for the response partial after creating a home expense.
	UpdateCarExp    string // UpdateHomeExp is the name for the response partial after updating a home expense.
	DeleteCarExp    string // DeleteHomeExp is the name for the response partial after deleting a home expense.
	RegisterSuccess string
}

// HTMLTemplates groups all template names used throughout the application.
// This provides a centralized and organized way to reference templates for rendering.
type HTMLTemplates struct {
	Root       string          // Root is the name for the root template that often includes other templates.
	Pages      *Pages          // Pages holds a collection of full page template names.
	Responses  *Responses      // Responses holds a collection of HTMX response partial template names.
	Components *HTMXComponents // Components holds a collection of reusable HTMX component template names.
}

var pages = &Pages{
	Index:    "index-page",
	Register: "register-page",
	Login:    "login-page",
	House:    "house-page",
	Car:      "car-page",
}

var components = &HTMXComponents{
	Header:             "header",
	Footer:             "footer",
	HouseExpRow:        "house-exp-row",
	CarExpRow:          "car-exp-row",
	CreateHouseExpForm: "create-house-exp-form",
	EditHouseExpForm:   "edit-house-exp-form",
	CreateCarExpForm:   "create-car-exp-form",
	EditCarExpForm:     "edit-car-exp-form",
	TotalCard:          "total-card",
	HighestCard:        "highest-card",
	Modal:              "modal",
	ModalSuccess:       "success-modal",
	ModalError:         "error-modal",
	ModalConfirm:       "confirm-modal",
	Chart:              "exp-chart",
	HouseCurrent:       "house-current",
	CarSummary:         "car-summary",
	Dialog:             "dialog",
	Search:             "search",
	SearchResultsHouse: "search-results-house",
}

// responses initializes the Responses struct with specific template identifiers.
var responses = &Responses{
	CreateHouseExp:  "create-house-exp",
	DeleteHouseExp:  "delete-house-exp",
	CreateCarExp:    "create-car-exp",
	DeleteCarExp:    "delete-car-exp",
	RegisterSuccess: "register-success",
}

// Templates is the main exported variable that provides access to all
// organized template names. It should be imported and used
// globally for consistent template referencing.
var Templates = &HTMLTemplates{
	Root:       "root",
	Pages:      pages,
	Responses:  responses,
	Components: components,
}
