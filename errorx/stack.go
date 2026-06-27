package errorx

import "runtime"

const defaultStackDepth = 32

// Frame is a single caller frame captured by WithStack.
type Frame struct {
	Function string `json:"function"`
	File     string `json:"file"`
	Line     int    `json:"line"`
}

func captureStack(skip int, depth int) []uintptr {
	if depth <= 0 {
		depth = defaultStackDepth
	}
	pcs := make([]uintptr, depth)
	n := runtime.Callers(skip, pcs)
	return pcs[:n]
}

func framesFromPCs(pcs []uintptr) []Frame {
	if len(pcs) == 0 {
		return nil
	}
	frames := runtime.CallersFrames(pcs)
	out := make([]Frame, 0, len(pcs))
	for {
		frame, more := frames.Next()
		out = append(out, Frame{Function: frame.Function, File: frame.File, Line: frame.Line})
		if !more {
			break
		}
	}
	return out
}
