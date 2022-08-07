package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
)

const (
	defaultPort = 18092
)

type Config struct {
	Port       uint32
	GitRepoURL string
}

func FromEnv() (conf Config, err error) {
	getEnv := func(key string) (val string, empty bool) {
		val = os.Getenv(key)
		if val == "" {
			return val, true
		}
		return val, false
	}

	portStr, empty := getEnv("PORT")
	if empty {
		conf.Port = defaultPort
	} else {
		port, err := strconv.ParseInt(portStr, 10, 32)
		if err != nil {
			return conf, fmt.Errorf("PORT value %q is invalid: %v", portStr, err)
		}
		conf.Port = uint32(port)
	}

	conf.GitRepoURL, empty = getEnv("GIT_REPO_URL")
	if empty {
		return conf, errors.New("GIT_REPO_URL cannot be empty")
	}

	return conf, nil
}
