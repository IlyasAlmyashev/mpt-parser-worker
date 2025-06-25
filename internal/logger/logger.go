package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger interface defines the logging methods we need.
type Logger interface {
	Debugf(template string, args ...interface{})
	Infof(template string, args ...interface{})
	Warnf(template string, args ...interface{})
	Errorf(template string, args ...interface{})
	Fatalf(template string, args ...interface{})
	Close() error
}

type zapLogger struct {
	logger  *zap.SugaredLogger
	syncFn  func() error
	closers []func() error // Список функций закрытия
	mu      sync.Mutex     // Мьютекс для безопасного доступа к closers
}

// generateLogFileName создает имя файла для логов с временной меткой.
func generateLogFileName() string {
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	return filepath.Join("logs", fmt.Sprintf("app_%s.log", timestamp))
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

	// Create a custom console encoder for Windows
	consoleEncoder := zapcore.NewConsoleEncoder(encoderConfig)

	// Настраиваем вывод
	var outputPaths []string
	var writers []zapcore.WriteSyncer
	var closers []func() error

	// Всегда добавляем stdout
	writers = append(writers, zapcore.AddSync(os.Stdout))

	// Setup output syncer
	if runtime.GOOS == "windows" {
		// Use file output on Windows to avoid console sync issues
		// outputPaths = []string{"logs/app.log", "stdout"}
		// Create a logs directory if it doesn't exist
		if err := os.MkdirAll("logs", 0750); err != nil {
			return nil, fmt.Errorf("failed to create logs directory: %w", err)
		}

		// Генерируем имя файла с временной меткой
		logFileName := generateLogFileName()
		outputPaths = append(outputPaths, logFileName)

		file, err := os.OpenFile(logFileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
		if err != nil {
			return nil, fmt.Errorf("failed to open log file: %w", err)
		}

		writers = append(writers, zapcore.AddSync(file))
		closers = append(closers, file.Close) // Сохраняем функцию закрытия файла
	}

	// Create a core with multiple outputs
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
			// Sync всегда возвращает успех, но внутренне пытается выполнить синхронизацию
			// Это предотвращает ошибки на Windows
			_ = sugar.Sync()
			return nil
		},
		closers: closers,
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

func (l *zapLogger) Fatalf(template string, args ...interface{}) {
	l.logger.Fatalf(template, args...)
}

// Close закрывает все ресурсы логгера, включая файловые дескрипторы.
func (l *zapLogger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Сначала синхронизируем буферы
	_ = l.syncFn()

	// Затем закрываем все файлы
	var lastErr error
	for _, closer := range l.closers {
		if err := closer(); err != nil {
			lastErr = err // Сохраняем последнюю ошибку
		}
	}

	// Очищаем список закрывающих функций
	l.closers = nil

	return lastErr
}

// parseLogLevel converts a string level to zapcore.Level.
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
