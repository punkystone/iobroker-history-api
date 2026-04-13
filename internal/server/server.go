package server

import (
	"log/slog"
	"net/http"
	"time"
)

const timeout = 5

type Handler struct {
	logger *slog.Logger
}

func (handler *Handler) handleHistory(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("test"))
}

func StartServer(logger *slog.Logger) error {
	handler := &Handler{
		logger: logger,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/history", handler.handleHistory)

	server := &http.Server{
		Addr:              ":80",
		Handler:           mux,
		ReadHeaderTimeout: timeout * time.Second,
	}
	return server.ListenAndServe()
}
