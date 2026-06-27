package logx

import "context"

type noopLogger struct{}

func Noop() Logger { return noopLogger{} }

func (noopLogger) Debug(string, ...Field)               {}
func (noopLogger) Info(string, ...Field)                {}
func (noopLogger) Warn(string, ...Field)                {}
func (noopLogger) Error(string, ...Field)               {}
func (l noopLogger) With(...Field) Logger               { return l }
func (l noopLogger) Named(string) Logger                { return l }
func (l noopLogger) WithContext(context.Context) Logger { return l }
func (noopLogger) Enabled(LogLevel) bool                { return false }
func (noopLogger) Sync() error                          { return nil }
