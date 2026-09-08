package main

import (
	"fmt"
	"net/http"
	"url-shortner/internal/handlers"
)

func homeHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "URL Shortner backend")
}

func main() {
	http.HandleFunc("/", homeHandler)
	http.HandleFunc("POST /url/shorten", handlers.ShortenURLHandler)
	http.HandleFunc("GET /{code}", handlers.RedirectHandler)
	
	fmt.Println("Server running on http://localhost:8080")

	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		fmt.Println(err)
	}
}