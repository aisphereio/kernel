{{ range .Errors }}

{{ if .HasComment }}{{ .Comment }}{{ end -}}
func Is{{.CamelValue}}(err error) bool {
	if err == nil {
		return false
	}
	e := errorx.From(err)
	return e.Code() == errorx.Code({{ .Name }}_{{ .Value }}.String()) && e.HTTPStatus() == {{ .HTTPCode }}
}

{{ if .HasComment }}{{ .Comment }}{{ end -}}
func New{{ .CamelValue }}(message string, opts ...errorx.Option) *errorx.Error {
	return errorx.NewStatus(errorx.Code({{ .Name }}_{{ .Value }}.String()), {{ .HTTPCode }}, message, opts...)
}

{{ if .HasComment }}{{ .Comment }}{{ end -}}
func Error{{ .CamelValue }}(format string, args ...interface{}) *errorx.Error {
	return New{{ .CamelValue }}(fmt.Sprintf(format, args...))
}

{{- end }}
