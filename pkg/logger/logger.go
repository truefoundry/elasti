package logger

import (
	"fmt"

	"github.com/getsentry/sentry-go"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func NewLogger(env string, sentryEnabled bool) (*zap.Logger, error) {
	var config zap.Config

	if env == "prod" {
		encoderCfg := zap.NewProductionEncoderConfig()
		encoderCfg.TimeKey = "timestamp"
		encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder

		config = zap.NewProductionConfig()
		config.EncoderConfig = encoderCfg
	} else {
		encoderCfg := zap.NewDevelopmentEncoderConfig()
		encoderCfg.EncodeLevel = zapcore.CapitalColorLevelEncoder
		encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder
		encoderCfg.EncodeDuration = zapcore.StringDurationEncoder
		encoderCfg.StacktraceKey = "" // removes stack trace from error logs

		config = zap.Config{
			Level:            zap.NewAtomicLevelAt(zap.DebugLevel),
			Development:      true,
			Encoding:         "console",
			EncoderConfig:    encoderCfg,
			OutputPaths:      []string{"stdout"},
			ErrorOutputPaths: []string{"stderr"},
		}
	}

	logger, err := config.Build()
	if err != nil {
		return nil, fmt.Errorf("error creating logger: %w", err)
	}

	if sentryEnabled {
		return zap.New(&CustomCore{Core: logger.Core()}), nil
	}
	return logger, nil
}

type CustomCore struct {
	zapcore.Core
}

func (c *CustomCore) Check(entry zapcore.Entry, checked *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if c.Enabled(entry.Level) {
		return checked.AddCore(entry, c)
	}
	return checked
}

func (c *CustomCore) Write(entry zapcore.Entry, fields []zapcore.Field) error {
	if entry.Level >= zapcore.ErrorLevel {
		sentry.WithScope(func(scope *sentry.Scope) {
			context := make(sentry.Context) // map[string]interface{}
			var err error

			// Convert Zap fields to Sentry context
			for _, field := range fields {
				switch field.Type {
				case zapcore.StringType:
					context[field.Key] = field.String
				case zapcore.Int64Type, zapcore.Int32Type, zapcore.Int16Type, zapcore.Int8Type:
					context[field.Key] = field.Integer
				case zapcore.ErrorType:
					fieldErr := field.Interface.(error)
					context[field.Key] = fieldErr.Error()
					if err == nil { // Using only the first error
						err = fieldErr
					}
				default:
					context[field.Key] = field.String
				}
			}

			scope.SetLevel(sentry.LevelError)
			scope.SetContext("details", context)

			stacktrace := sentry.NewStacktrace()
			stacktrace.Frames = stacktrace.Frames[:len(stacktrace.Frames)-4]

			exception := sentry.Exception{
				Type:       entry.Message,
				Stacktrace: stacktrace,
			}
			if err != nil {
				exception.Value = err.Error()
			} else {
				exception.Value = entry.Message
			}

			event := &sentry.Event{
				Message: entry.Message,
				Level:   sentry.LevelError,
				Exception: []sentry.Exception{
					exception,
				},
			}

			sentry.CaptureEvent(event)
		})
	}
	return c.Core.Write(entry, fields)
}
