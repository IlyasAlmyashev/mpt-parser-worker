package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	KafkaHost         string `env:"KAFKA_HOST"`
	KafkaTopic        string `env:"KAFKA_TOPIC"`
	LogLevel          string `env:"LOG_LEVEL"`
	Timeout           int    `env:"TIMEOUT"`
	EnableKaspi       bool   `env:"ENABLE_KASPI"`
	EnableOzon        bool   `env:"ENABLE_OZON"`
	EnableWildberries bool   `env:"ENABLE_WILDBERRIES"`
	AppPort           int    `env:"APP_PORT"`
}

func Load() (*Config, error) {
	err := godotenv.Load()
	if err != nil {
		log.Println("Error loading .env file")
		return nil, err
	}

	config := &Config{
		KafkaHost:         getEnvAsString("KAFKA_HOST", "localhost:29092"),
		KafkaTopic:        getEnvAsString("KAFKA_TOPIC", "products.raw"),
		LogLevel:          getEnvAsString("LOG_LEVEL", "info"),
		Timeout:           getEnvAsInt("TIMEOUT", 30),
		EnableKaspi:       getEnvAsBool("ENABLE_KASPI", false),
		EnableOzon:        getEnvAsBool("ENABLE_OZON", false),
		EnableWildberries: getEnvAsBool("ENABLE_WILDBERRIES", false),
		AppPort:           getEnvAsInt("APP_PORT", 8081),
	}
	log.Printf("Loaded configuration: %+v\n", config)

	return config, nil
}

func getEnvAsString(key string, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

func getEnvAsInt(key string, defaultValue int) int {
	value, err := strconv.Atoi(os.Getenv(key))
	if err != nil {
		return defaultValue
	}
	return value
}

func getEnvAsBool(key string, defaultValue bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value == "true"
}
