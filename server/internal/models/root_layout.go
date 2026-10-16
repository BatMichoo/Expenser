package models

type RootLayout struct {
	TemplateName    string
	TemplateContent any
	HeaderOpts      *HeaderOptions
	Lang            string
	Year            int
	IsLoggedIn      bool
	HasFlash        bool // a flash error waits to be shown as a popup
}
