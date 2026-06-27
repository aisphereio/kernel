package http

import (
	"net/http"
	"net/url"

	encoding "github.com/aisphereio/kernel/encodingx"
	"github.com/aisphereio/kernel/encodingx/form"
	"github.com/aisphereio/kernel/errorx"
)

func bindQuery(vars url.Values, target any) error {
	if err := encoding.GetCodec(form.Name).Unmarshal([]byte(vars.Encode()), target); err != nil {
		return errorx.BadRequest("REQUEST_BIND_FAILED", "request binding failed", errorx.WithCause(err), errorx.WithMetadata("binding_error", err.Error()))
	}
	return nil
}

func bindForm(req *http.Request, target any) error {
	if err := req.ParseForm(); err != nil {
		return err
	}
	if err := encoding.GetCodec(form.Name).Unmarshal([]byte(req.Form.Encode()), target); err != nil {
		return errorx.BadRequest("REQUEST_BIND_FAILED", "request binding failed", errorx.WithCause(err), errorx.WithMetadata("binding_error", err.Error()))
	}
	return nil
}
