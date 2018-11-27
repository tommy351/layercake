package main

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"github.com/ansel1/merry"
	"github.com/fatih/color"
	"github.com/sirupsen/logrus"
)

const logKeyPrefix = "prefix"

// nolint: gochecknoglobals
var (
	logger      = logrus.New()
	colorGray   = color.New(color.FgHiBlack)
	colorDebug  = colorGray
	colorWarn   = color.New(color.FgYellow)
	colorInfo   = color.New(color.FgGreen)
	colorError  = color.New(color.FgRed)
	colorPrefix = color.New(color.FgCyan, color.Bold)
)

func initLogger() (err error) {
	formatter := &logFormatter{}

	if globalOptions.Debug {
		merry.SetVerboseDefault(true)
		logger.SetLevel(logrus.DebugLevel)
		logger.SetReportCaller(true)
		formatter.TimestampFormat = "15:04:05"
		formatter.ShowTimestamp = true
	}

	logger.SetFormatter(formatter)

	return merry.Wrap(err)
}

type logFormatter struct {
	TimestampFormat string
	ShowTimestamp   bool
}

func (l *logFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	buf := entry.Buffer

	if buf == nil {
		buf = &bytes.Buffer{}
	}

	c := l.getColor(entry)

	// Write timestamp
	if l.ShowTimestamp {
		buf.WriteString(colorGray.Sprint(entry.Time.Format(l.TimestampFormat)))
		buf.WriteByte(' ')
	}

	// Write level
	buf.WriteString(c.Sprint(strings.ToUpper(entry.Level.String()[0:4])))
	buf.WriteByte(' ')

	// Write prefix
	if prefix, ok := entry.Data[logKeyPrefix]; ok {
		buf.WriteString(colorPrefix.Sprintf("%s:", prefix))
		buf.WriteByte(' ')
	}

	// Write message
	buf.WriteString(strings.TrimSpace(entry.Message))

	// Sort data
	var keys []string

	for k := range entry.Data {
		if k != logKeyPrefix {
			keys = append(keys, k)
		}
	}

	sort.Strings(keys)

	// Print data
	for _, k := range keys {
		buf.WriteByte(' ')
		buf.WriteString(c.Sprint(k))

		if v := l.formatValue(entry.Data[k]); v != "" {
			buf.WriteByte('=')
			buf.WriteString(v)
		}
	}

	buf.WriteByte('\n')
	return buf.Bytes(), nil
}

func (*logFormatter) getColor(entry *logrus.Entry) *color.Color {
	switch entry.Level {
	case logrus.DebugLevel, logrus.TraceLevel:
		return colorDebug
	case logrus.WarnLevel:
		return colorWarn
	case logrus.ErrorLevel, logrus.FatalLevel, logrus.PanicLevel:
		return colorError
	default:
		return colorInfo
	}
}

func (*logFormatter) formatValue(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	default:
		return fmt.Sprint(v)
	}
}
