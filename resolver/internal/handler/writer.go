package handler

import (
	"fmt"
	"net/http"
)

type responseWriter struct {
	http.ResponseWriter
	statusCode int
	body       []byte
}

func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{ResponseWriter: w, statusCode: 0}
}

func (rw *responseWriter) WriteHeader(statusCode int) {
	rw.statusCode = statusCode
	rw.ResponseWriter.WriteHeader(statusCode)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	rw.body = append(rw.body, b...)
	res, err := rw.ResponseWriter.Write(b)
	if err != nil {
		return res, fmt.Errorf("Write: %w", err)
	}
	return res, nil
}

func (rw *responseWriter) Header() http.Header {
	return rw.ResponseWriter.Header()
}
