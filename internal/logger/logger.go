package logger

import "go.uber.org/zap"

type Logger interface {
	Infof(template string, args ...interface{})
	Errorf(template string, args ...interface{})
	Debugf(template string, args ...interface{})
	Sync() error
}

func New() (Logger, error) {
	log, err := zap.NewProduction()
	if err != nil {
		return nil, err
	}
	return log.Sugar(), nil
}
