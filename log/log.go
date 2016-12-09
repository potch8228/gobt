package log

import (
	"log"
	"os"
)

var btlog = NewLogger()

type Logger struct {
	logger *log.Logger
	Enable bool
}

func NewLogger() *Logger {
	e := false
	if env := os.Getenv("DEBUG"); env == "1" {
		e = !e
	}
	return &Logger{
		logger: log.New(os.Stdout, "Debug: ", log.LstdFlags),
		Enable: e,
	}
}

func (l *Logger) Debug(args ...interface{}) {
	for _, v := range args {
		l.logger.Println(v)
	}
}

func Debug(args ...interface{}) {
	if btlog.Enable {
		btlog.Debug(args...)
	}
}

func ForceDebug(args ...interface{}) {
	btlog.Debug(args...)
}

func (l *Logger) Fatal(args ...interface{}) {
	e := len(args)
	var pv *interface{}
	for i, v := range args {
		if i == e {
			pv = &v
			break
		}
		l.logger.Println(v)
	}
	l.logger.Fatalln(pv)
}

func Fatal(args ...interface{}) {
	btlog.Fatal(args...)
}
