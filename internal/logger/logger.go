package logger

import (
	"os"
	"strings"

	"github.com/sirupsen/logrus"
)

var (
	// Log is the global logger instance
	Log *logrus.Logger
)

// Initialize sets up the logger with proper formatting and level
func Initialize() {
	Log = logrus.New()
	Log.SetOutput(os.Stdout)
	Log.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:    true,
		TimestampFormat:  "2006-01-02 15:04:05",
		DisableColors:    false,
		DisableTimestamp: false,
	})

	// Set default log level
	logLevel := logrus.InfoLevel

	// Check environment variable for log level
	if envLevel := os.Getenv("TICTACTOE_LOG_LEVEL"); envLevel != "" {
		switch strings.ToUpper(envLevel) {
		case "DEBUG":
			logLevel = logrus.DebugLevel
		case "INFO":
			logLevel = logrus.InfoLevel
		case "WARN", "WARNING":
			logLevel = logrus.WarnLevel
		case "ERROR":
			logLevel = logrus.ErrorLevel
		}
	}

	Log.SetLevel(logLevel)
	Info("Logger initialized", Fields{"level": logLevel.String()})
}

// Fields shorthand for logrus.Fields
type Fields logrus.Fields

// Error logs a message at level Error
func Error(msg string, fields Fields) {
	if fields == nil {
		Log.Error(msg)
	} else {
		Log.WithFields(logrus.Fields(fields)).Error(msg)
	}
}

// Info logs a message at level Info
func Info(msg string, fields Fields) {
	if fields == nil {
		Log.Info(msg)
	} else {
		Log.WithFields(logrus.Fields(fields)).Info(msg)
	}
}

// Warn logs a message at level Warn
func Warn(msg string, fields Fields) {
	if fields == nil {
		Log.Warn(msg)
	} else {
		Log.WithFields(logrus.Fields(fields)).Warn(msg)
	}
}

// Debug logs a message at level Debug
func Debug(msg string, fields Fields) {
	if fields == nil {
		Log.Debug(msg)
	} else {
		Log.WithFields(logrus.Fields(fields)).Debug(msg)
	}
}

// Fatal logs a message at level Fatal then the process will exit with status set to 1
func Fatal(msg string, fields Fields) {
	if fields == nil {
		Log.Fatal(msg)
	} else {
		Log.WithFields(logrus.Fields(fields)).Fatal(msg)
	}
}
