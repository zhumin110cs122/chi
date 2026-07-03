package middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"
	"time"
)

func TestTimeout(t *testing.T) {
	r := NewRouter()
	r.Use(Timeout(50 * time.Millisecond))
	r.Get("/timeout", func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
			return
		case <-time.After(100 * time.Millisecond):
			w.Write([]byte("done"))
		}
	})

	ts := httptest.NewServer(r)
	defer ts.Close()

	res, err := http.Get(ts.URL + "/timeout")
	if err != nil {
		ttFatal(err)
	}
	if res.StatusCode != http.StatusGatewayTimeout {
		t.Errorf("expected status 504, got %d", res.StatusCode)
	}
}

func TestTimeoutCancelsContext(t *testing.T) {
	r := NewRouter()
	r.Use(Timeout(50 * time.Millisecond))

	ctxCancelled := make(chan struct{})
	r.Get("/timeout", func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
			close(ctxCancelled)
		case <-time.After(200 * time.Millisecond):
		}
	})

	ts := httptest.NewServer(r)
	defer ts.Close()

	start := time.Now()
	res, err := http.Get(ts.URL + "/timeout")
	if err != nil {
		t.Fatal(err)
	}
	duration := time.Since(start)

	if res.StatusCode != http.StatusGatewayTimeout {
		t.Errorf("expected status 504, got %d", res.StatusCode)
	}

	select {
	case <-ctxCancelled:
		// Success
	case <-time.After(100 * time.Millisecond):
		t.Error("context was not cancelled in time")
	}

	if duration > 100*time.Millisecond {
		t.Errorf("request took too long: %v", duration)
	}
}

func TestTimeoutDuringStreaming(t *testing.T) {
	r := NewRouter()
	r.Use(Timeout(50 * time.Millisecond))

	ctxCancelled := make(chan struct{})
	r.Get("/stream", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		select {
		case <-r.Context().Done():
			close(ctxCancelled)
		case <-time.After(200 * time.Millisecond):
		}
	})

	ts := httptest.NewServer(r)
	defer ts.Close()

	res, err := http.Get(ts.URL + "/stream")
	if err == nil {
		defer res.Body.Close()
		_, _ = io.ReadAll(res.Body)
	}

	select {
	case <-ctxCancelled:
		// Success
	case <-time.After(100 * time.Millisecond):
		t.Error("context was not cancelled during streaming")
	}
}

func TestTimeoutGoroutineLeak(t *testing.T) {
	initialGoroutines := runtime.NumGoroutine()

	r := NewRouter()
	r.Use(Timeout(10 * time.Millisecond))

	handlerDone := make(chan struct{})
	r.Get("/timeout", func(w http.ResponseWriter, r *http.Request) {
		defer close(handlerDone)
		select {
		case <-r.Context().Done():
		case <-time.After(100 * time.Millisecond):
		}
	})

	ts := httptest.NewServer(r)
	defer ts.Close()

	res, err := http.Get(ts.URL + "/timeout")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()

	<-handlerDone

	time.Sleep(50 * time.Millisecond)

	finalGoroutines := runtime.NumGoroutine()
	if finalGoroutines > initialGoroutines+2 {
		t.Errorf("possible goroutine leak: started with %d, ended with %d", initialGoroutines, finalGoroutines)
	}
}
