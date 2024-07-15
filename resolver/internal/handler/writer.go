package handler

import "net/http"

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
	return rw.ResponseWriter.Write(b)
}

func (rw *responseWriter) Header() http.Header {
	return rw.ResponseWriter.Header()
}
