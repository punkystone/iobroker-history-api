package server

import (
	"encoding/json"
	"go_test/internal/history"
	"log/slog"
	"net/http"
	"strconv"
	"time"
)

const timeout = 5

type Handler struct {
	logger         *slog.Logger
	historyService *history.HistoryService
}

type HistoryRequest struct {
	ID    string `json:"id"`
	Start int64  `json:"start"`
	End   int64  `json:"end"`
	Count int    `json:"count"`
}
type Point struct {
	Timestamp float64 `json:"timestamp"`
	Value     float64 `json:"value"`
}

type HistoryResponse struct {
	Success bool    `json:"success"`
	Data    []Point `json:"data"`
}

func (handler *Handler) handleHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	defer func() {
		if err := r.Body.Close(); err != nil {
			handler.logger.Error("close error", "error", err)
		}
	}()
	request := &HistoryRequest{}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	points, err := handler.historyService.GetHistory(request.ID, strconv.Itoa(request.Count), float64(request.Start), float64(request.End))
	if err != nil {
		handler.logger.Error("error requesting history", "error", err)
		writeError(w, http.StatusBadRequest, "error requesting history")
		return
	}
	mappedPoints := make([]Point, len(points))
	for i := range points {
		source := &points[i]
		destination := &mappedPoints[i]
		destination.Timestamp = source.Ts
		destination.Value = source.Val
	}
	response := HistoryResponse{
		Success: true,
		Data:    mappedPoints,
	}
	writeJSON(w, http.StatusOK, response)
}

func StartServer(logger *slog.Logger, historyService *history.HistoryService) error {
	handler := &Handler{
		logger:         logger,
		historyService: historyService,
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
