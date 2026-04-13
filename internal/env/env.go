package env

import (
	"errors"
	"os"
)

type Env struct {
	Host     string
	Instance string
}

func CheckEnv() (*Env, error) {
	host, exists := os.LookupEnv("IO_BROKER_HOST")
	if !exists {
		return nil, errors.New("IO_BROKER_HOST environment variable not set")
	}
	instance, exists := os.LookupEnv("IO_BROKER_INSTANCE")
	if !exists {
		return nil, errors.New("IO_BROKER_INSTANCE environment variable not set")
	}
	env := &Env{
		Host:     host,
		Instance: instance,
	}
	return env, nil
}
