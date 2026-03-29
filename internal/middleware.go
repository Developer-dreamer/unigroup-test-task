package internal

import (
	"context"
	"github.com/google/uuid"
	"net/http"
	"runtime/debug"
)

type RecoveryManager struct {
	logger Logger
}

func NewRecoveryManager(logger Logger) *RecoveryManager {
	return &RecoveryManager{
		logger: logger,
	}
}

func (rm *RecoveryManager) Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				if err == http.ErrAbortHandler {
					panic(err)
				}

				stack := debug.Stack()

				rm.logger.ErrorContext(r.Context(), "PANIC RECOVERED",
					"error", err,
					"stack", stack,
					"method", r.Method,
					"path", r.URL.Path,
				)

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"error": "Internal Server Error"}`))
			}
		}()

		next.ServeHTTP(w, r)
	})
}

func TracingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		traceID := r.Header.Get("uber-trace-id")

		if traceID == "" {
			traceID = uuid.New().String()
		}

		ctx := context.WithValue(r.Context(), "trace_id", traceID)

		w.Header().Set("uber-trace-id", traceID)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
