package env

import (
	"errors"
	"os"
	"strconv"
)

type Env struct {
	Host     string
	Instance string
	Debug    bool
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
	debug, exists := os.LookupEnv("DEBUG")
	if !exists {
		return nil, errors.New("DEBUG environment variable not set")
	}
	debugParsed, err := strconv.ParseBool(debug)
	if err != nil {
		return nil, errors.New("DEBUG environment variable not a boolean")
	}
	env := &Env{
		Host:     host,
		Instance: instance,
		Debug:    debugParsed,
	}
	return env, nil
}
