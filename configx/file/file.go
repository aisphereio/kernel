package file

import (
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/aisphereio/kernel/configx"
)

var _ configx.Source = (*file)(nil)

type file struct {
	path string
}

// NewSource new a file source.
func NewSource(path string) configx.Source {
	return &file{path: path}
}

func (f *file) loadFile(path string) (*configx.KeyValue, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	return &configx.KeyValue{
		Key:    info.Name(),
		Format: format(info.Name()),
		Value:  data,
	}, nil
}

// supportedExts lists file extensions that are treated as config files.
// Files with other extensions (e.g. .pub, .pem, .cert) are silently skipped.
var supportedExts = map[string]bool{
	".yaml": true,
	".yml":  true,
	".json": true,
	".toml": true,
	".proto": true,
}

func (f *file) loadDir(path string) (kvs []*configx.KeyValue, err error) {
	files, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	for _, file := range files {
		// ignore hidden files and directories
		if file.IsDir() || strings.HasPrefix(file.Name(), ".") {
			continue
		}
		// skip files with unsupported extensions (e.g. .pub, .pem, .cert)
		ext := strings.ToLower(filepath.Ext(file.Name()))
		if !supportedExts[ext] {
			continue
		}
		kv, err := f.loadFile(filepath.Join(path, file.Name()))
		if err != nil {
			return nil, err
		}
		kvs = append(kvs, kv)
	}
	return
}

func (f *file) Load() (kvs []*configx.KeyValue, err error) {
	fi, err := os.Stat(f.path)
	if err != nil {
		return nil, err
	}
	if fi.IsDir() {
		return f.loadDir(f.path)
	}
	kv, err := f.loadFile(f.path)
	if err != nil {
		return nil, err
	}
	return []*configx.KeyValue{kv}, nil
}

func (f *file) Watch() (configx.Watcher, error) {
	return newWatcher(f)
}
