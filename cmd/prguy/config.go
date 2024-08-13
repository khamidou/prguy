package main

import (
	"github.com/zalando/go-keyring"
	"os/user"
)

const keychainService = "prguy"

type Config struct {
	OAuthToken string
}

func (c *Config) exists() bool {
	username, err := user.Current()
	if err != nil {
		return false
	}

	_, err = keyring.Get(keychainService, username.Username)
	if err != nil {
		return false
	}

	return true
}

func (c *Config) save() error {
	username, err := user.Current()
	if err != nil {
		return err
	}

	err = keyring.Set(keychainService, username.Username, c.OAuthToken)
	if err != nil {
		return err
	}

	return nil
}

func (c *Config) load() error {
	username, err := user.Current()
	if err != nil {
		return err
	}

	secret, err := keyring.Get(keychainService, username.Username)
	if err != nil {
		return err
	}

	c.OAuthToken = secret
	return nil
}
