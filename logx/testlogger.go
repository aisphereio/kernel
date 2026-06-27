package logx

import (
	"context"
	"reflect"
	"sync"
	"testing"
)

type testLoggerCore struct {
	mu      sync.Mutex
	entries []Entry
}

type TestLogger struct {
	core   *testLoggerCore
	prefix []Field
	name   string
}

func NewTestLogger(t testing.TB) *TestLogger {
	if t != nil {
		t.Helper()
	}
	return &TestLogger{core: &testLoggerCore{}}
}

func (l *TestLogger) ensureCore() {
	if l.core == nil {
		l.core = &testLoggerCore{}
	}
}

func (l *TestLogger) Debug(msg string, fields ...Field) { l.log(DebugLevel, msg, fields...) }
func (l *TestLogger) Info(msg string, fields ...Field)  { l.log(InfoLevel, msg, fields...) }
func (l *TestLogger) Warn(msg string, fields ...Field)  { l.log(WarnLevel, msg, fields...) }
func (l *TestLogger) Error(msg string, fields ...Field) { l.log(ErrorLevel, msg, fields...) }

func (l *TestLogger) With(fields ...Field) Logger {
	l.ensureCore()
	return &TestLogger{
		core:   l.core,
		name:   l.name,
		prefix: append(append([]Field(nil), l.prefix...), fields...),
	}
}

func (l *TestLogger) Named(name string) Logger {
	l.ensureCore()
	return &TestLogger{
		core:   l.core,
		name:   name,
		prefix: append(append([]Field(nil), l.prefix...), String("module", name)),
	}
}

func (l *TestLogger) WithContext(context.Context) Logger { return l }
func (l *TestLogger) Enabled(LogLevel) bool              { return true }
func (l *TestLogger) Sync() error                        { return nil }

func (l *TestLogger) Entries() []Entry {
	l.ensureCore()
	l.core.mu.Lock()
	defer l.core.mu.Unlock()
	return append([]Entry(nil), l.core.entries...)
}

func (l *TestLogger) AssertLogged(t testing.TB, msg string, fields ...Field) {
	t.Helper()
	for _, entry := range l.Entries() {
		if entry.Message != msg {
			continue
		}
		if fieldsContained(entry.Fields, fields) {
			return
		}
	}
	t.Fatalf("expected log %q with fields %#v, got %#v", msg, fields, l.Entries())
}

func (l *TestLogger) log(level LogLevel, msg string, fields ...Field) {
	l.ensureCore()
	l.core.mu.Lock()
	defer l.core.mu.Unlock()
	all := append(append([]Field(nil), l.prefix...), fields...)
	l.core.entries = append(l.core.entries, Entry{Level: level, Message: msg, Fields: all})
}

func fieldsContained(got []Field, want []Field) bool {
	for _, wf := range want {
		found := false
		for _, gf := range got {
			if gf.Key == wf.Key && reflect.DeepEqual(gf.Value, wf.Value) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
