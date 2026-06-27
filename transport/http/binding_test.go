package http

import (
	"errors"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/aisphereio/kernel/errorx"
)

type (
	testBind struct {
		Name string `json:"name"`
		URL  string `json:"url"`
	}
	testBind2 struct {
		Age int `json:"age"`
	}
)

func TestBindQuery(t *testing.T) {
	type args struct {
		vars   url.Values
		target any
	}

	tests := []struct {
		name string
		args args
		err  error
		want any
	}{
		{
			name: "test",
			args: args{
				vars:   map[string][]string{"name": {"kernel"}, "url": {"https://kernel.aisphere.io/"}},
				target: &testBind{},
			},
			err:  nil,
			want: &testBind{"kernel", "https://kernel.aisphere.io/"},
		},
		{
			name: "test1",
			args: args{
				vars:   map[string][]string{"age": {"kernel"}, "url": {"https://kernel.aisphere.io/"}},
				target: &testBind2{},
			},
			err: errorx.BadRequest("REQUEST_BIND_FAILED", "request binding failed"),
		},
		{
			name: "test2",
			args: args{
				vars:   map[string][]string{"age": {"1"}, "url": {"https://kernel.aisphere.io/"}},
				target: &testBind2{},
			},
			err:  nil,
			want: &testBind2{Age: 1},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := bindQuery(tt.args.vars, tt.args.target)
			if !errors.Is(err, tt.err) {
				t.Fatalf("bindQuery() error = %v, err %v", err, tt.err)
			}
			if err == nil && !reflect.DeepEqual(tt.args.target, tt.want) {
				t.Errorf("bindQuery() target = %v, want %v", tt.args.target, tt.want)
			}
		})
	}
}

func TestBindForm(t *testing.T) {
	type args struct {
		req    *http.Request
		target any
	}

	tests := []struct {
		name string
		args args
		err  error
		want *testBind
	}{
		{
			name: "error not nil",
			args: args{
				req:    &http.Request{Method: http.MethodPost},
				target: &testBind{},
			},
			err:  errors.New("missing form body"),
			want: nil,
		},
		{
			name: "error is nil",
			args: args{
				req: &http.Request{
					Method: http.MethodPost,
					Header: http.Header{"Content-Type": {"application/x-www-form-urlencoded; param=value"}},
					Body:   io.NopCloser(strings.NewReader("name=kernel&url=https://kernel.aisphere.io/")),
				},
				target: &testBind{},
			},
			err:  nil,
			want: &testBind{"kernel", "https://kernel.aisphere.io/"},
		},
		{
			name: "error BadRequest",
			args: args{
				req: &http.Request{
					Method: http.MethodPost,
					Header: http.Header{"Content-Type": {"application/x-www-form-urlencoded; param=value"}},
					Body:   io.NopCloser(strings.NewReader("age=a")),
				},
				target: &testBind2{},
			},
			err:  errorx.BadRequest("REQUEST_BIND_FAILED", "request binding failed"),
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := bindForm(tt.args.req, tt.args.target)
			assertBindError(t, err, tt.err)
			if err == nil && !reflect.DeepEqual(tt.args.target, tt.want) {
				t.Errorf("bindForm() target = %v, want %v", tt.args.target, tt.want)
			}
		})
	}
}

func assertBindError(t *testing.T, got, want error) {
	t.Helper()
	if want == nil {
		if got != nil {
			t.Fatalf("expected nil error, got %v", got)
		}
		return
	}
	if got == nil {
		t.Fatalf("expected error %v, got nil", want)
	}
	if errorx.IsKernelError(want) {
		if !errors.Is(got, want) {
			t.Fatalf("expected kernel error %v, got %v", want, got)
		}
		if errorx.MessageOf(got) != errorx.MessageOf(want) {
			t.Fatalf("expected error message %q, got %q", errorx.MessageOf(want), errorx.MessageOf(got))
		}
		return
	}
	if got.Error() != want.Error() {
		t.Fatalf("expected error %q, got %q", want.Error(), got.Error())
	}
}
