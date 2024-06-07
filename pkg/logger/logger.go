package logger

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func NewLogger(env string) (logger *zap.Logger, err error) {
	if env == "prod" {
		logger, err = zap.NewProduction()
		if err != nil {
			return nil, err
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
	return logger, nil
}
