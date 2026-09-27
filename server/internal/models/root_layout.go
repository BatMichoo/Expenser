package models

type RootLayout struct {
	TemplateName    string
	TemplateContent any
	HeaderOpts      *HeaderOptions
	Lang            string
	Year            int
	IsLoggedIn      bool
}
