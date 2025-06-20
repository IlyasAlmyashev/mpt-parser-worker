package logger

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger interface defines the logging methods we need.
type Logger interface {
	Debugf(template string, args ...interface{})
	Infof(template string, args ...interface{})
	Warnf(template string, args ...interface{})
	Errorf(template string, args ...interface{})
	Sync() error
}

type zapLogger struct {
	logger *zap.SugaredLogger
	syncFn func() error
}

// New creates a new logger with the specified log level.
func New(logLevel string) (Logger, error) {
	level, err := parseLogLevel(logLevel)
	if err != nil {
		level = zapcore.InfoLevel
	}

	// Create custom encoder config
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "ts",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	// Create custom console encoder for Windows
	consoleEncoder := zapcore.NewConsoleEncoder(encoderConfig)

	// Setup output syncer
	var outputPaths []string
	if runtime.GOOS == "windows" {
		// Use file output on Windows to avoid console sync issues
		outputPaths = []string{"logs/app.log", "stdout"}
		// Create logs directory if it doesn't exist
		if err := os.MkdirAll("logs", 0755); err != nil {
			return nil, fmt.Errorf("failed to create logs directory: %w", err)
		}
	} else {
		outputPaths = []string{"stdout"}
	}

	// Create writers for each output path
	var writers []zapcore.WriteSyncer
	for _, path := range outputPaths {
		if path == "stdout" {
			writers = append(writers, zapcore.AddSync(os.Stdout))
		} else {
			file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				return nil, fmt.Errorf("failed to open log file: %w", err)
			}
			writers = append(writers, zapcore.AddSync(file))
		}
	}

	// Create core with multiple outputs
	core := zapcore.NewTee(
		zapcore.NewCore(
			consoleEncoder,
			zapcore.NewMultiWriteSyncer(writers...),
			level,
		),
	)

	// Create logger
	logger := zap.New(
		core,
		zap.AddCaller(),
		zap.AddCallerSkip(1),
		zap.AddStacktrace(zapcore.ErrorLevel),
	)

	// Create sugar logger with sync handler
	sugar := logger.Sugar()

	return &zapLogger{
		logger: sugar,
		syncFn: func() error {
			if runtime.GOOS == "windows" {
				// Ignore sync errors on Windows
				_ = sugar.Sync()
				return nil
			}
			return sugar.Sync()
		},
	}, nil
}

func (l *zapLogger) Debugf(template string, args ...interface{}) {
	l.logger.Debugf(template, args...)
}

func (l *zapLogger) Infof(template string, args ...interface{}) {
	l.logger.Infof(template, args...)
}

func (l *zapLogger) Warnf(template string, args ...interface{}) {
	l.logger.Warnf(template, args...)
}

func (l *zapLogger) Errorf(template string, args ...interface{}) {
	l.logger.Errorf(template, args...)
}

func (l *zapLogger) Sync() error {
	return l.syncFn()
}

// parseLogLevel converts a string level to zapcore.Level
func parseLogLevel(level string) (zapcore.Level, error) {
	switch strings.ToLower(level) {
	case "debug":
		return zapcore.DebugLevel, nil
	case "info":
		return zapcore.InfoLevel, nil
	case "warn":
		return zapcore.WarnLevel, nil
	case "error":
		return zapcore.ErrorLevel, nil
	case "dpanic":
		return zapcore.DPanicLevel, nil
	case "panic":
		return zapcore.PanicLevel, nil
	case "fatal":
		return zapcore.FatalLevel, nil
	default:
		return zapcore.InfoLevel, fmt.Errorf("unknown log level: %s", level)
	}
}
