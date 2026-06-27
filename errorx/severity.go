package errorx

import (
	"fmt"
	"reflect"
	"strings"
)

// Severity is a log-neutral severity hint. logx maps it to its concrete logger
// level. errorx deliberately does not depend on zap, slog or GoFr logging.
type Severity string

const (
	SeverityDebug Severity = "debug"
	SeverityInfo  Severity = "info"
	SeverityWarn  Severity = "warn"
	SeverityError Severity = "error"
)

func (s Severity) String() string { return string(s) }

func defaultSeverityByStatus(status int) Severity {
	switch status {
	case HTTPStatusOK:
		return SeverityInfo
	case HTTPStatusClientClosedRequest:
		return SeverityDebug
	case HTTPStatusBadRequest, HTTPStatusUnauthorized, HTTPStatusForbidden, HTTPStatusNotFound, HTTPStatusRequestTimeout:
		return SeverityInfo
	case HTTPStatusConflict, HTTPStatusTooManyRequests:
		return SeverityWarn
	default:
		if status >= HTTPStatusInternalServerError {
			return SeverityError
		}
		return SeverityInfo
	}
}

// severityFromForeignLogLevel recognizes common LogLevel() results without
// importing the foreign logger package. It supports string/fmt.Stringer levels
// and the GoFr numeric convention DEBUG=1, INFO=2, NOTICE=3, WARN=4, ERROR=5.
func severityFromForeignLogLevel(err error) (Severity, bool) {
	if err == nil {
		return "", false
	}
	v := reflect.ValueOf(err)
	if !v.IsValid() {
		return "", false
	}
	m := v.MethodByName("LogLevel")
	if !m.IsValid() || m.Type().NumIn() != 0 || m.Type().NumOut() != 1 {
		return "", false
	}
	out := m.Call(nil)[0].Interface()
	switch x := out.(type) {
	case string:
		return parseSeverity(x)
	case fmt.Stringer:
		return parseSeverity(x.String())
	}
	rv := reflect.ValueOf(out)
	switch rv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		switch rv.Int() {
		case 1:
			return SeverityDebug, true
		case 2, 3:
			return SeverityInfo, true
		case 4:
			return SeverityWarn, true
		case 5, 6:
			return SeverityError, true
		}
	}
	return "", false
}

func parseSeverity(s string) (Severity, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return SeverityDebug, true
	case "info", "notice":
		return SeverityInfo, true
	case "warn", "warning":
		return SeverityWarn, true
	case "error", "fatal", "panic":
		return SeverityError, true
	default:
		return "", false
	}
}
