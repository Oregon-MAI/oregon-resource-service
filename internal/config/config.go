package config

import (
	"errors"
	"flag"
	"os"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	Env      string   `yaml:"env" env-default:"local"`
	GRPC     GRPC     `yaml:"grpc"`
	Tracer   Tracer   `yaml:"tracer"`
	Database Database `yaml:"database"`
}

type GRPC struct {
	Booking ServiceConfig `yaml:"booking"`
	Public  ServiceConfig `yaml:"public"`
}

type ServiceConfig struct {
	Port int `yaml:"port"`
}

type Tracer struct {
	EndPoint    string  `yaml:"end-point" env:"END_POINT"`
	Insecure    bool    `yaml:"insecure" env:"INSECURE"`
	SampleRatio float64 `yaml:"sample-ratio" env:"SAMPLE_RATION"`
}

type Database struct {
	Host     string `yaml:"host"`
	Port     string `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Name     string `yaml:"name"`
	SSLMode  string `yaml:"ssl_mode" env-default:"disable"`
}

func MustLoad() *Config {
	configPath := fetchConfigPath()

	if configPath == "" {
		panic("config path is empty")
	}

	if _, err := os.Stat(configPath); errors.Is(err, os.ErrNotExist) {
		panic("config file does not exists: " + configPath)
	}

	var cfg Config

	if err := cleanenv.ReadConfig(configPath, &cfg); err != nil {
		panic("config path is empty: " + err.Error())
	}

	return &cfg
}

func fetchConfigPath() string {
	var path string

	flag.StringVar(&path, "config", "", "path to config file")
	flag.Parse()

	if path == "" {
		path = os.Getenv("CONFIG_PATH")
	}

	return path
}
