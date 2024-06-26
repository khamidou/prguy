package main

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"
)

type Config struct {
	OAuthToken      string
	Scope           string
	LastUpdatedTime string
}

func (c *Config) getConfigPath() (string, error) {
	dirname, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(dirname, ".prguy"), nil
}

func (c *Config) exists() bool {
	configPath, err := c.getConfigPath()
	_, err = os.Stat(configPath)
	return err == nil
}

func (c *Config) save() error {
	configPath, err := c.getConfigPath()
	file, err := os.Create(configPath)
	if err != nil {
		return err
	}

	defer file.Close()
	c.LastUpdatedTime = time.Now().String()
	jsonData, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	_, err = file.Write(jsonData)
	if err != nil {
		return err
	}

	return nil
}

func (c *Config) load() error {
	configPath, err := c.getConfigPath()
	file, err := os.Open(configPath)
	if err != nil {
		return err
	}

	defer file.Close()
	jsonData, err := ioutil.ReadAll(file)
	if err != nil {
		return err
	}

	return json.Unmarshal(jsonData, c)
}
