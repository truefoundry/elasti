package logger

import (
	"fmt"
	"strconv"
	"strings"

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
		logger = logger.WithOptions(zap.Hooks(func(entry zapcore.Entry) error {
			if entry.Level >= zapcore.ErrorLevel {
				event := &sentry.Event{
					Message:   entry.Message,
					Level:     sentry.LevelError,
					Timestamp: entry.Time,
				}

				if entry.Stack != "" {
					stackTrace, err := parseStackTrace(entry.Stack)
					if err != nil {
						return err
					}
					event.Exception = []sentry.Exception{
						{
							Value:      entry.Message,
							Type:       "error",
							Stacktrace: stackTrace,
						},
					}
				}

				sentry.CaptureEvent(event)
			}
			return nil
		}))
	}

	return logger, nil
}

func parseStackTrace(stack string) (*sentry.Stacktrace, error) {
	var frames []sentry.Frame
	lines := strings.Split(stack, "\n")

	for i := 0; i < len(lines); i += 2 {
		funcName := strings.TrimSpace(lines[i])
		fileName := strings.TrimSpace(lines[i+1])

		parts := strings.Split(fileName, ":")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid stack trace line: %s", fileName)
		}

		lineNumber, err := strconv.Atoi(parts[1])
		if err != nil {
			return nil, fmt.Errorf("invalid stack trace line: %s", fileName)
		}

		frame := sentry.Frame{
			Function: funcName,
			Filename: parts[0],
			Lineno:   lineNumber,
		}

		frames = append(frames, frame)
	}

	// Reverse the order of the frames for sentry to display them in the correct order
	for i, j := 0, len(frames)-1; i < j; i, j = i+1, j-1 {
		frames[i], frames[j] = frames[j], frames[i]
	}

	return &sentry.Stacktrace{Frames: frames}, nil
}
