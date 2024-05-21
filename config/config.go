package config

import (
	"encoding/json"
	"os"
)

type Config struct {
	NCloud NCloud `json:"ncloud"`
	DB     DB     `json:"db"`
}

type NCloud struct {
	SecretKey string `json:"secretKey"`
	AccessKey string `json:"key"`
}

type DB struct {
	Bucket string `json:"bucket"`
	Region string `json:"region"`
}

var gConfig *Config

func OpenConfig() (*Config, error) {
	var config Config
	if gConfig == nil {
		if data, err := os.ReadFile("config.json"); err != nil {
			return nil, err
		} else {
			if err := json.Unmarshal(data, &config); err != nil {
				return nil, err
			} else {
				gConfig = &config
				return gConfig, nil
			}
		}
	} else {
		return gConfig, nil
	}
}
