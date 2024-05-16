package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/kelseyhightower/envconfig"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type config struct {
	MaxIdleProxyConns        int `split_words:"true" default:"1000"`
	MaxIdleProxyConnsPerHost int `split_words:"true" default:"100"`
}

func main() {
	ctx := context.Background()
	encoderConfig := zap.NewDevelopmentEncoderConfig()
	encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	encoderConfig.EncodeTime = zapcore.RFC3339TimeEncoder
	encoderConfig.EncodeDuration = zapcore.StringDurationEncoder
	consoleEncoder := zapcore.NewConsoleEncoder(encoderConfig)
	core := zapcore.NewCore(consoleEncoder, zapcore.AddSync(os.Stdout), zapcore.DebugLevel)
	logger := zap.New(core)

	InformerLockTimeout := time.Duration(30 * time.Second)

	var env config
	if err := envconfig.Process("", &env); err != nil {
		log.Fatal("Failed to process env: ", err)
	}
	logger.Debug("config",
		zap.Int("MaxIdleProxyConns", env.MaxIdleProxyConns),
		zap.Int("MaxIdleProxyConnsPerHost", env.MaxIdleProxyConnsPerHost))

	InitK8sUtil(logger)
	InitInformer(logger, InformerLockTimeout)
	transport := NewProxyAutoTransport(logger, env.MaxIdleProxyConns, env.MaxIdleProxyConnsPerHost)
	throttler := NewThrottler(ctx, logger, K8sUtil)
	handler := NewHandler(ctx, logger, transport, *throttler)
	http.HandleFunc("/", handler.ServeHTTP)
	logger.Info("Reverse Proxy Server starting at ", zap.String("port", "8012"))
	if err := http.ListenAndServe(":8012", nil); err != nil {
		logger.Fatal("ListenAndServe Failed: ", zap.Error(err))
	}
}
