package logger

import (
	"log"
	"os"
)

// Logger defines a simple interface for logging.
// This allows for different logging implementations to be used.
type Logger interface {
	Debugf(format string, args ...interface{})
	Infof(format string, args ...interface{})
	Warnf(format string, args ...interface{})
	Errorf(format string, args ...interface{})
	Fatalf(format string, args ...interface{})
	Debug(args ...interface{})
	Info(args ...interface{})
	Warn(args ...interface{})
	Error(args ...interface{})
	Fatal(args ...interface{})
}

// stdLogger is a basic implementation of the Logger interface using the standard log package.

type stdLogger struct {
	logger *log.Logger
}

// NewStdLogger creates a new logger that writes to os.Stdout with standard log flags.
func NewStdLogger() Logger {
	return &stdLogger{
		logger: log.New(os.Stdout, "", log.LstdFlags),
	}
}

func (l *stdLogger) Debugf(format string, args ...interface{}) {
	l.logger.Printf("[DEBUG] "+format, args...)
}

func (l *stdLogger) Infof(format string, args ...interface{}) {
	l.logger.Printf("[INFO] "+format, args...)
}

func (l *stdLogger) Warnf(format string, args ...interface{}) {
	l.logger.Printf("[WARN] "+format, args...)
}

func (l *stdLogger) Errorf(format string, args ...interface{}) {
	l.logger.Printf("[ERROR] "+format, args...)
}

func (l *stdLogger) Fatalf(format string, args ...interface{}) {
	l.logger.Fatalf("[FATAL] "+format, args...)
}

func (l *stdLogger) Debug(args ...interface{}) {
	logArgs := append([]interface{}{"[DEBUG]"}, args...)
	l.logger.Println(logArgs...)
}

func (l *stdLogger) Info(args ...interface{}) {
	logArgs := append([]interface{}{"[INFO]"}, args...)
	l.logger.Println(logArgs...)
}

func (l *stdLogger) Warn(args ...interface{}) {
	logArgs := append([]interface{}{"[WARN]"}, args...)
	l.logger.Println(logArgs...)
}

func (l *stdLogger) Error(args ...interface{}) {
	logArgs := append([]interface{}{"[ERROR]"}, args...)
	l.logger.Println(logArgs...)
}

func (l *stdLogger) Fatal(args ...interface{}) {
	logArgs := append([]interface{}{"[FATAL]"}, args...)
	l.logger.Fatalln(logArgs...)
}
