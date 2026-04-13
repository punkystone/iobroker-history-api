package main

import (
	"go_test/internal/env"
	"go_test/internal/history"
	"go_test/internal/server"
	"log/slog"
	"os"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	env, err := env.CheckEnv()
	if err != nil {
		panic(err)
	}
	historyService := history.NewHistoryService(logger, env.Host, env.Instance)
	historyService.Connect()
	err = server.StartServer(logger)
	if err != nil {
		panic(err)
	}
}
