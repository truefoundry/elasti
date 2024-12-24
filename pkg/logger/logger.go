package logger

import (
	"fmt"
	"github.com/getsentry/sentry-go"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func NewLogger(env string, sentryEnabled bool) (*zap.Logger, error) {
	var logger *zap.Logger

	if env == "prod" {
		var err error
		logger, err = zap.NewProduction()
		if err != nil {
			return nil, fmt.Errorf("failed to create logger: %w", err)
		}
	} else {
		encoderConfig := zap.NewDevelopmentEncoderConfig()
		encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		encoderConfig.EncodeTime = zapcore.RFC3339TimeEncoder
		encoderConfig.EncodeDuration = zapcore.StringDurationEncoder
		consoleEncoder := zapcore.NewConsoleEncoder(encoderConfig)

		core := zapcore.NewCore(consoleEncoder, zapcore.AddSync(os.Stdout), zapcore.DebugLevel)
		logger = zap.New(core)
	}

	if sentryEnabled {
		logger = logger.WithOptions(zap.Hooks(func(entry zapcore.Entry) error {
			if entry.Level == zapcore.ErrorLevel {
				sentry.CaptureMessage(entry.Message)
			}
			return nil
		}))
	}

	return logger, nil
}
