package logger

import (
	"log"
	"os"
)

type CustomLogger struct {
	logger *log.Logger
}

func NewCustomLogger(file *os.File) *CustomLogger {
	return &CustomLogger{
		logger: log.New(file, "", log.LstdFlags),
	}
}

func (l *CustomLogger) Info(args ...interface{}) {
	l.logger.Print(args...)
}

func (l *CustomLogger) Infoln(args ...interface{}) {
	l.logger.Println(args...)
}

func (l *CustomLogger) Infof(format string, args ...interface{}) {
	l.logger.Printf(format, args...)
}
