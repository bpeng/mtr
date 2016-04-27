package main

import (
	"html/template"
	"log"
)

var (
	borderTemplate  *template.Template
)

func init() {
	loadTemplates()
}

func loadTemplates() {
	log.Println("Loading templates.")
	borderTemplate = template.Must(template.New("t").ParseFiles("tmpl/demo.html", "tmpl/border.html"))
	log.Println("Done loading templates.")
}