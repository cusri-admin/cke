package log

import (
	"fmt"
	"os"
	"strings"
)

const (
	ENV_LOG_LEVEL = "CKE_LOG_LEVEL"
)

type Level int

const (
	ERROR   Level = 0
	WARNING Level = 1
	INFO    Level = 2
	DEBUG   Level = 3
)

func (l Level) String() string {
	switch l {
	case ERROR:
		return "ERROR"
	case WARNING:
		return "WARNING"
	case INFO:
		return "INFO"
	case DEBUG:
		return "DEBUG"
	default:
		return "UNKNOWN"
	}
}

//Set the output level of logs.
func ToLevel(level string) Level {
	switch strings.ToUpper(level) {
	case ERROR.String():
		return ERROR
	case WARNING.String():
		return WARNING
	case INFO.String():
		return INFO
	case DEBUG.String():
		return DEBUG
	}
	return INFO
}

// curLevel controls whether level we should ouput logs.
var curLevel Level

var loggers []*Logger

func init() {
	loggers = make([]*Logger, 4)

	//golog.Lshortfile
	loggers[ERROR] = newLogger(os.Stderr, ERROR)
	loggers[WARNING] = newLogger(os.Stderr, WARNING)
	loggers[INFO] = newLogger(os.Stdout, INFO)
	loggers[DEBUG] = newLogger(os.Stdout, DEBUG)

	curLevel = INFO
}

// logf calls log.Printf prefixed with level.
func logf(level Level, format string, v ...interface{}) {
	if level <= curLevel {
		loggers[level].output(fmt.Sprintf(format, v...))
	}
}

// log calls log.Print prefixed with level.
func log(level Level, v ...interface{}) {
	if level <= curLevel {
		loggers[level].output(fmt.Sprint(v...))
	}
}

//Set the output level of logs.
func SetLevel(level Level) {
	curLevel = level
}

func PrintLevel() {
	var buf []byte
	buf = append(buf, "Log level = "...)
	buf = append(buf, curLevel.String()...)
	buf = append(buf, '\n')
	loggers[INFO].out.Write(buf)
}

//Get the output level of logs.
func GetLevel() Level {
	return curLevel
}

// Debug is equivalent to log.Print(), but prefixed with "[DEBUG] ".
func Debug(v ...interface{}) { log(DEBUG, v...) }

// Debugf is equivalent to log.Printf(), but prefixed with "[DEBUG] ".
func Debugf(format string, v ...interface{}) { logf(DEBUG, format, v...) }

// Info is equivalent to log.Print, but prefixed with "[INFO] ".
func Info(v ...interface{}) { log(INFO, v...) }

// Infof is equivalent to log.Printf, but prefixed with "[INFO] ".
func Infof(format string, v ...interface{}) { logf(INFO, format, v...) }

// Warning is equivalent to log.Print, but prefixed with "[WARNING] ".
func Warning(v ...interface{}) { log(WARNING, v...) }

// Warningf is equivalent to log.Printf, but prefixed with "[WARNING] ".
func Warningf(format string, v ...interface{}) { logf(WARNING, format, v...) }

// Error is equivalent to log.Print, but prefixed with "[ERROR] ".
func Error(v ...interface{}) { log(ERROR, v...) }

// Errorf is equivalent to log.Printf, but prefixed with "[ERROR] ".
func Errorf(format string, v ...interface{}) { logf(ERROR, format, v...) }
