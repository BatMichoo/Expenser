package models

type HeaderOptions struct {
	IsLoggedIn bool
	IsOOB      bool
	Lang       string
	LogoutURL  string // the home server's log out (a POST)
}
