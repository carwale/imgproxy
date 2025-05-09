package logger

import (
	"os"

	logrus "github.com/sirupsen/logrus"

	"github.com/imgproxy/imgproxy/v3/config/configurators"
)

func init() {
	// Configure logrus so it can be used before Init().
	// Structured formatter is a compromise between JSON and pretty formatters.
	logrus.SetOutput(os.Stdout)
	logrus.SetFormatter(&structuredFormatter{})
}

func Init() error {
	logrus.SetOutput(os.Stdout)

	logFormat := "json"
	logLevel := "warn"

	configurators.String(&logFormat, "IMGPROXY_LOG_FORMAT")
	configurators.String(&logLevel, "IMGPROXY_LOG_LEVEL")

	switch logFormat {
	case "structured":
		logrus.SetFormatter(&structuredFormatter{})
	case "json":
		logrus.SetFormatter(&logrus.JSONFormatter{})
	case "gcp":
		logrus.SetFormatter(&logrus.JSONFormatter{
			FieldMap: logrus.FieldMap{
				"level": "severity",
				"msg":   "message",
			},
		})
	default:
		logrus.SetFormatter(newPrettyFormatter())
	}

	levelLogLevel, err := logrus.ParseLevel(logLevel)
	if err != nil {
		levelLogLevel = logrus.InfoLevel
	}

	logrus.SetLevel(levelLogLevel)

	return nil
}
