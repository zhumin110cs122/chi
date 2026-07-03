package middleware

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"sync"
	"time"
)

// Timeout is a middleware that cancels ctx after duration.
func Timeout(timeout time.Duration) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), timeout)

			r = r.WithContext(ctx)

			done := make(chan struct{})
			panicChan := make(chan interface{}, 1)

			tw := &timeoutWriter{
				w: w,
			}

			go func() {
				defer func() {
					if p := recover(); p != nil {
						panicChan <- p
					}
					close(done)
				}()
				next.ServeHTTP(tw, r)
			}()

			select {
			case p := <-panicChan:
				cancel()
				panic(p)
			case <-done:
				cancel()
			case <-ctx.Done():
				cancel()
				tw.mu.Lock()
				tw.timedOut = true
				wroteHeader := tw.wroteHeader
				tw.mu.Unlock()

				if !wroteHeader {
					w.WriteHeader(http.StatusGatewayTimeout)
				} else {
					panic(http.ErrAbortHandler)
				}
			}
		}
		return http.HandlerFunc(fn)
	}
}

type timeoutWriter struct {
	w           http.ResponseWriter
	mu          sync.Mutex
	wroteHeader bool
	timedOut    bool
}

func (tw *timeoutWriter) Header() http.Header {
	return tw.w.Header()
}

func (tw *timeoutWriter) Write(b []byte) (int, error) {
	tw.mu.Lock()
	if tw.timedOut {
		tw.mu.Unlock()
		return 0, http.ErrHandlerTimeout
	}
	tw.wroteHeader = true
	tw.mu.Unlock()
	return tw.w.Write(b)
}

func (tw *timeoutWriter) WriteHeader(code int) {
	tw.mu.Lock()
	if tw.timedOut {
		tw.mu.Unlock()
		return
	}
	tw.wroteHeader = true
	tw.mu.Unlock()
	tw.w.WriteHeader(code)
}

func (tw *timeoutWriter) Flush() {
	tw.mu.Lock()
	if tw.timedOut {
		tw.mu.Unlock()
		return
	}
	tw.mu.Unlock()
	if f, ok := tw.w.(http.Flusher); ok {
		f.Flush()
	}
}

func (tw *timeoutWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	tw.mu.Lock()
	if tw.timedOut {
		tw.mu.Unlock()
		return nil, nil, http.ErrHandlerTimeout
	}
	tw.mu.Unlock()
	if h, ok := tw.w.(http.Hijacker); ok {
		return h.Hijack()
	}
	return nil, nil, errors.New("chi: ResponseWriter does not support Hijacker interface")
}

func (tw *timeoutWriter) Push(target string, opts *http.PushOptions) error {
	tw.mu.Lock()
	if tw.timedOut {
		tw.mu.Unlock()
		return http.ErrHandlerTimeout
	}
	tw.mu.Unlock()
	if p, ok := tw.w.(http.Pusher); ok {
		return p.Push(target, opts)
	}
	return http.ErrNotSupported
}

func (tw *timeoutWriter) ReadFrom(src io.Reader) (int64, error) {
	tw.mu.Lock()
	if tw.timedOut {
		tw.mu.Unlock()
		return 0, http.ErrHandlerTimeout
	}
	tw.wroteHeader = true
	tw.mu.Unlock()
	if rf, ok := tw.w.(io.ReaderFrom); ok {
		return rf.ReadFrom(src)
	}
	return io.Copy(tw.w, src)
}
