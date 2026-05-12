package models

import "html/template"

type ModalContent struct {
	Title   string
	Message string
	Lang    string
}

type ModalConfirmContent struct {
	Title    string
	Target   string
	Method   string
	Endpoint template.URL
	Message  string
	Lang     string
}
