package httpx

import (
	"net/http"

	"github.com/namezzy/goscaffoldforlevi/internal/config"
)

// NewServer builds the HTTP server.
//
// Every timeout is set explicitly. The zero value of http.Server has no
// timeouts at all, which means a single slow client can hold a connection
// open indefinitely; that is the most common way a Go service falls over.
//
//   - ReadHeaderTimeout defends against Slowloris.
//   - ReadTimeout and WriteTimeout bound the whole exchange. WriteTimeout must
//     exceed the request timeout, or the connection dies before the timeout
//     middleware can report the error.
//   - IdleTimeout bounds keep-alive connections.
func NewServer(cfg config.HTTP, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              cfg.Addr(),
		Handler:           handler,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
		MaxHeaderBytes:    cfg.MaxHeaderBytes,
	}
}
