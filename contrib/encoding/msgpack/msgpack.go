package msgpack

import (
	"github.com/vmihailenco/msgpack/v5"

	encoding "github.com/aisphereio/kernel/encodingx"
)

// Name is the name registered for the msgpack compressor.
const Name = "msgpack"

func init() {
	encoding.RegisterCodec(codec{})
}

// codec is a Codec implementation with msgpack.
type codec struct{}

func (codec) Marshal(v any) ([]byte, error) {
	return msgpack.Marshal(v)
}

func (codec) Unmarshal(data []byte, v any) error {
	return msgpack.Unmarshal(data, v)
}

func (codec) Name() string {
	return Name
}
