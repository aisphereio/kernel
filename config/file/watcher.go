package file

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/aisphereio/kernel/config"
)

var _ config.Watcher = (*watcher)(nil)

type watcher struct {
	f         *file
	fw        *fsnotify.Watcher
	watchPath string

	ctx    context.Context
	cancel context.CancelFunc
}

func newWatcher(f *file) (config.Watcher, error) {
	fw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	watchPath := f.path
	if fi, err := os.Stat(f.path); err != nil {
		return nil, err
	} else if !fi.IsDir() {
		watchPath = filepath.Dir(f.path)
	}
	if err := fw.Add(watchPath); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &watcher{f: f, fw: fw, watchPath: watchPath, ctx: ctx, cancel: cancel}, nil
}

func (w *watcher) Next() ([]*config.KeyValue, error) {
	for {
		select {
		case <-w.ctx.Done():
			return nil, w.ctx.Err()
		case event, ok := <-w.fw.Events:
			if !ok {
				return nil, fsnotify.ErrClosed
			}
			if !w.match(event.Name) {
				continue
			}
			if event.Has(fsnotify.Rename) {
				if _, err := os.Stat(event.Name); err == nil || os.IsExist(err) {
					if err := w.fw.Add(event.Name); err != nil {
						return nil, err
					}
				}
			}
			fi, err := os.Stat(w.f.path)
			if err != nil {
				return nil, err
			}
			path := w.f.path
			if fi.IsDir() {
				path = filepath.Join(w.f.path, filepath.Base(event.Name))
			}
			time.Sleep(time.Millisecond)
			kv, err := w.f.loadFile(path)
			if err != nil {
				return nil, err
			}
			return []*config.KeyValue{kv}, nil
		case err, ok := <-w.fw.Errors:
			if !ok {
				return nil, fsnotify.ErrClosed
			}
			return nil, err
		}
	}
}

func (w *watcher) match(name string) bool {
	if samePath(w.watchPath, w.f.path) {
		rel, err := filepath.Rel(filepath.Clean(w.f.path), filepath.Clean(name))
		if err != nil {
			return false
		}
		return rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
	}
	return samePath(name, w.f.path)
}

func samePath(a, b string) bool {
	a = filepath.Clean(a)
	b = filepath.Clean(b)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

func (w *watcher) Stop() error {
	w.cancel()
	return w.fw.Close()
}
