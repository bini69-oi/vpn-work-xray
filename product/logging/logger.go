package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"regexp"
	"strings"
)

type Logger struct {
	base      *slog.Logger
	module    string
	requestID string
}

func New(filePath string) (*Logger, error) {
	var w io.Writer = os.Stdout
	if filePath == "" {
		handler := slog.NewJSONHandler(w, &slog.HandlerOptions{
			Level: slog.LevelInfo,
			ReplaceAttr: func(_ []string, attr slog.Attr) slog.Attr {
				switch attr.Key {
				case slog.TimeKey:
					attr.Key = "timestamp"
				case slog.LevelKey:
					if level, ok := attr.Value.Any().(slog.Level); ok {
						attr.Value = slog.StringValue(strings.ToUpper(level.String()))
					}
				}
				return attr
			},
		})
		return &Logger{
			base:      slog.New(handler),
			module:    "app",
			requestID: "system",
		}, nil
	}
	// #nosec G304 -- log file path is operator-provided deployment config.
	f, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, err
	}
	w = io.MultiWriter(os.Stdout, f)
	handler := slog.NewJSONHandler(w, &slog.HandlerOptions{
		Level: slog.LevelInfo,
		ReplaceAttr: func(_ []string, attr slog.Attr) slog.Attr {
			switch attr.Key {
			case slog.TimeKey:
				attr.Key = "timestamp"
			case slog.LevelKey:
				if level, ok := attr.Value.Any().(slog.Level); ok {
					attr.Value = slog.StringValue(strings.ToUpper(level.String()))
				}
			}
			return attr
		},
	})
	return &Logger{
		base:      slog.New(handler),
		module:    "app",
		requestID: "system",
	}, nil
}

func (l *Logger) WithModule(module string) *Logger {
	if module == "" {
		module = "app"
	}
	return &Logger{base: l.base, module: module, requestID: l.requestID}
}

func (l *Logger) WithRequestID(requestID string) *Logger {
	if requestID == "" {
		requestID = "system"
	}
	return &Logger{base: l.base, module: l.module, requestID: requestID}
}

func (l *Logger) Debugf(format string, args ...any) {
	l.log(slog.LevelDebug, formatMessage(format, args...))
}

func (l *Logger) Infof(format string, args ...any) {
	l.log(slog.LevelInfo, formatMessage(format, args...))
}

func (l *Logger) Warnf(format string, args ...any) {
	l.log(slog.LevelWarn, formatMessage(format, args...))
}

func (l *Logger) Errorf(format string, args ...any) {
	l.log(slog.LevelError, formatMessage(format, args...))
}

func (l *Logger) log(level slog.Level, msg string, attrs ...any) {
	module := l.module
	if module == "" {
		module = "app"
	}
	requestID := l.requestID
	if requestID == "" {
		requestID = "system"
	}
	args := []any{"module", module, "request_id", requestID}
	args = append(args, attrs...)
	l.base.Log(context.Background(), level, msg, args...)
}

func formatMessage(format string, args ...any) string {
	msg := format
	if len(args) == 0 {
		return redactSensitive(msg)
	}
	msg = fmt.Sprintf(format, args...)
	return redactSensitive(msg)
}

var sensitivePattern = regexp.MustCompile(`(?i)(token|password|secret|privatekey|publickey|shortid)=([^\s]+)`)

func redactSensitive(msg string) string {
	if msg == "" {
		return msg
	}
	msg = sensitivePattern.ReplaceAllString(msg, "$1=[REDACTED]")
	msg = strings.ReplaceAll(msg, "replace-me", "[REDACTED]")
	return msg
}
