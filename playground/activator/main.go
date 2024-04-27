package main

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type Response struct {
	Message string `json:"message"`
}

func pingHandler(w http.ResponseWriter, r *http.Request) {
	response := Response{Message: "pong"}
	jsonResponse, err := json.Marshal(response)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(jsonResponse)
}

func main() {
	// Define a new HTTP server mux
	mux := http.NewServeMux()

	// Attach the pingHandler to the /ping endpoint
	mux.HandleFunc("/ping", pingHandler)

	// Start the HTTP server on port 9009
	port := "9009"
	fmt.Printf("Server running on port %s...\n", port)
	err := http.ListenAndServe(":"+port, mux)
	if err != nil {
		fmt.Printf("Error starting server: %s\n", err)
	}
}

