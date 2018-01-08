package logger

import (
	"github.com/sirupsen/logrus"
)

// utcFormatter is a custom logrus formatter that converts the time for all
// entries to UTC.
type utcFormatter struct {
	logrus.Formatter
}

// Format implements logrus.Formatter.Format().
func (f utcFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	entry.Time = entry.Time.UTC()
	return f.Formatter.Format(entry)
}

// LogConfig sets the format of the default logger.
func LogConfig() {
	logrus.SetFormatter(&utcFormatter{
		&logrus.TextFormatter{
			TimestampFormat:  "2006-01-02T15:04:05.000000000Z07:00",
			FullTimestamp:    true,
			DisableColors:    false,
			DisableTimestamp: false,
			DisableSorting:   false,
		},
	})
}
