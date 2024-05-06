package main

import (
	"context"
	"errors"
	"net/http"

	"go.uber.org/zap"
)

type Handler struct {
	logger    *zap.Logger
	throttler Throttler
	transport http.RoundTripper
}

func NewHandler(ctx context.Context, logger *zap.Logger, transport http.RoundTripper, throttle Throttler) *Handler {
	return &Handler{
		throttler: throttle,
		logger:    logger,
		transport: transport,
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	h.logger.Debug("Sending request for try")
	if tryErr := h.throttler.Try(ctx, func(dest string) error {
		// If the try is successful, how do we want to handle the reuqest.
		h.logger.Debug("Try successful, processing request")
		w.WriteHeader(http.StatusOK)
		return nil
	}); tryErr != nil {
		h.logger.Error("throttler try error: ", zap.Error(tryErr))

		if errors.Is(tryErr, context.DeadlineExceeded) {
			http.Error(w, tryErr.Error(), http.StatusServiceUnavailable)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
	}

}
