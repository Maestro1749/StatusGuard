package logger

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func NewLogger() (*zap.Logger, error) {
	if err := os.MkdirAll("logs", 0755); err != nil {
		return nil, err
	}

	config := zap.NewProductionConfig()

	config.OutputPaths = []string{
		"stdout",
		"logs/app.log",
	}

	config.EncoderConfig.TimeKey = "timestamp"
	config.EncoderConfig.EncodeTime = zapcore.TimeEncoderOfLayout("2006-01-02 15:04:05")

	return config.Build()
}
