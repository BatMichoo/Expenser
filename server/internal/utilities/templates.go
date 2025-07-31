package utilities

// Pages defines the names for full application pages.
type Pages struct {
	Index    string // Index is the name for the main index page template.
	Register string
	Login    string
	Home     string // Home is the name for the home-related expense page template.
	Car      string // Car is the name for the car-related expense page template.
}

// HTMXComponents defines the names for reusable HTMX-specific UI components.
type HTMXComponents struct {
	Header            string // Header is the name for the application's header component.
	Footer            string // Footer is the name for the application's footer component.
	CreateHomeExpForm string // CreateExpForm is the name for the expense creation form component.
	NewHomeExp        string // NewExp is the name for the new expense component (often a row or card).
	EditHomeExpForm   string // EditExpForm is the name for the expense editing form component.
	CreateCarExpForm  string // CreateExpForm is the name for the expense creation form component.
	NewCarExp         string // NewExp is the name for the new expense component (often a row or card).
	EditCarExpForm    string // EditExpForm is the name for the expense editing form component.
	HomeExpRow        string
	CarExpRow         string
	TotalCard         string
	HighestCard       string
	ServerError       string
}

// Responses defines the names for specific HTMX partial responses.
// These are often fragments returned by HTMX requests that swap content on the page.
type Responses struct {
	CreateHomeExp   string // CreateHomeExp is the name for the response partial after creating a home expense.
	UpdateHomeExp   string // UpdateHomeExp is the name for the response partial after updating a home expense.
	DeleteHomeExp   string // DeleteHomeExp is the name for the response partial after deleting a home expense.
	CreateCarExp    string // CreateHomeExp is the name for the response partial after creating a home expense.
	UpdateCarExp    string // UpdateHomeExp is the name for the response partial after updating a home expense.
	DeleteCarExp    string // DeleteHomeExp is the name for the response partial after deleting a home expense.
	LoginSuccess    string
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
	Home:     "home-page",
	Car:      "car-page",
}

var components = &HTMXComponents{
	Header:            "header",
	Footer:            "footer",
	HomeExpRow:        "home-exp-row",
	CarExpRow:         "car-exp-row",
	CreateHomeExpForm: "create-home-exp-form",
	EditHomeExpForm:   "edit-home-exp-form",
	CreateCarExpForm:  "create-car-exp-form",
	EditCarExpForm:    "edit-car-exp-form",
	TotalCard:         "total-card",
	HighestCard:       "highest-card",
	ServerError:       "server-error",
}

// responses initializes the Responses struct with specific template identifiers.
var responses = &Responses{
	CreateHomeExp:   "create-home-exp",
	DeleteHomeExp:   "delete-home-exp",
	CreateCarExp:    "create-car-exp",
	DeleteCarExp:    "delete-car-exp",
	LoginSuccess:    "login-success",
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
