package mediaexport

import "context"

// ProgressFunc receives export progress from fastRemux: done/total fetched
// chunks and the current phase — "fetch" (parallel chunk downloads, the bulk
// of the wall time), "concat" (joining chunks), "send" (copying the finished
// file to the client).
type ProgressFunc func(done, total int, phase string)

type progressKey struct{}

// WithProgress returns a context that makes exports running under it report
// progress to f. The HTTP layer uses this to expose a MEGA-style in-page
// progress bar without threading a callback through every vendor adapter
// (camera.Recorder's method signatures stay unchanged).
func WithProgress(ctx context.Context, f ProgressFunc) context.Context {
	return context.WithValue(ctx, progressKey{}, f)
}

// progressFrom extracts the reporter, or a no-op when none is attached.
func progressFrom(ctx context.Context) ProgressFunc {
	if f, ok := ctx.Value(progressKey{}).(ProgressFunc); ok && f != nil {
		return f
	}
	return func(int, int, string) {}
}
