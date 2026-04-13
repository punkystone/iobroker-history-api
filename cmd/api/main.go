package main

import (
	"go_test/internal/env"
	"go_test/internal/server"
	"log/slog"
	"os"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	_, err := env.CheckEnv()
	if err != nil {
		panic(err)
	}
	err = server.StartServer(logger)
	if err != nil {
		panic(err)
	}
}
