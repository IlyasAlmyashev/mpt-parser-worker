package logger

import "fmt"

type MockLogger struct {
	InfoMessages  []string
	ErrorMessages []string
}

func NewMockLogger() *MockLogger {
	return &MockLogger{
		InfoMessages:  make([]string, 0),
		ErrorMessages: make([]string, 0),
	}
}

func (m *MockLogger) Infof(template string, args ...interface{}) {
	m.InfoMessages = append(m.InfoMessages, fmt.Sprintf(template, args...))
}

func (m *MockLogger) Errorf(template string, args ...interface{}) {
	m.ErrorMessages = append(m.ErrorMessages, fmt.Sprintf(template, args...))
}

func (m *MockLogger) Debugf(template string, args ...interface{}) {}

func (m *MockLogger) Sync() error { return nil }
