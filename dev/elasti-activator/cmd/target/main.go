package main

import (
	"encoding/json"
	"net/http"

	"go.uber.org/zap"
)

type Response struct {
	Message string `json:"message"`
}

func main() {
	logger, _ := zap.NewDevelopment()
	logger.Debug("Starting service")

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		response := Response{
			Message: "Hello from the second service!",
		}
		jsonResponse, err := json.Marshal(response)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		// Print the request object individually
		logger.Debug("Request", zap.Any("host", r.Host),
			zap.Any("header", r.Header),
			zap.Any("method", r.Method),
			zap.Any("proto", r.Proto),
			zap.Any("Req URI", r.RequestURI),
			zap.Any("Req Body", r.Body),
			zap.Any("Req Remote Addr", r.RemoteAddr),
			zap.Any("Req URL", r.URL),
		)

		w.Write(jsonResponse)
	})

	logger.Info("Service started on port 8014")
	http.ListenAndServe(":8014", nil)
}
