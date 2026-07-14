package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	TelegramBotToken         string
	TelegramChatID           string
	CheckerWorkers           int
	SchedulerIntervalSeconds int
	AppPort                  int
	DatabaseURL              string
	RedisURL                 string
}

func MustLoad() *Config {
	_ = godotenv.Load()

	cfg := &Config{
		TelegramBotToken:         os.Getenv("TELEGRAM_BOT_TOKEN"),
		TelegramChatID:           os.Getenv("TELEGRAM_CHAT_ID"),
		CheckerWorkers:           getEnvAsInt("CHECKER_WORKERS", 5),
		SchedulerIntervalSeconds: getEnvAsInt("SCHEDULER_INTERVAL_SECONDS", 5),
		AppPort:                  getEnvAsInt("APP_PORT", 8080),
		DatabaseURL:              getEnvRequired("DATABASE_URL"),
		RedisURL:                 getEnvRequired("REDIS_URL"),
	}

	return cfg
}

func getEnvRequired(key string) string {
	value := os.Getenv(key)
	if value == "" {
		log.Fatalf("required env variable %s is not set", key)
	}

	return value
}

func getEnvAsInt(key string, defaultValue int) int {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}

	intValue, err := strconv.Atoi(value)
	if err != nil {
		log.Fatalf("env variable %s must be integer", key)
	}

	if intValue <= 0 {
		log.Fatalf("env variable %s must be greater than 0", key)
	}

	return intValue
}
