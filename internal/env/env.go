package env

import (
	"errors"
	"os"
)

type Env struct {
	Host string
}

func CheckEnv() (*Env, error) {
	host, exists := os.LookupEnv("IO_BROKER_HOST")
	if !exists {
		return nil, errors.New("IO_BROKER_HOST environment variable not set")
	}
	env := &Env{
		Host: host,
	}
	return env, nil
}
