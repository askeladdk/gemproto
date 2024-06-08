package gemproto

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"sync/atomic"
	"time"

	"github.com/askeladdk/gemproto/gemtext"
)

// ErrServerClosed is returned by Listen when the server has been closed.
var ErrServerClosed = errors.New("gemproto: server closed")

// Handler responds to a Gemini request.
type Handler interface {
	ServeGemini(ResponseWriter, *Request)
}

// HandlerFunc adapts a function to the Handler interface.
type HandlerFunc func(ResponseWriter, *Request)

// ServeGemini implements Handler.
func (f HandlerFunc) ServeGemini(w ResponseWriter, r *Request) {
	f(w, r)
}

// ResponseWriter is used to construct the response.
//
// WriteHeader sets the response header. It is not actually
// written until the first call to Write.
// The header will not be written if statusCode is set to a value lower than 10.
// This can be used to create CGI handlers that write the header manually.
type ResponseWriter interface {
	io.Writer
	WriteHeader(statusCode int, meta string)
}

type responseWriter struct {
	w           io.Writer
	statusCode  int
	metadata    string
	wroteHeader bool
}

func (rw *responseWriter) writeHeader() error {
	if !rw.wroteHeader {
		rw.wroteHeader = true
		if rw.statusCode >= 10 {
			return reply(rw.w, rw.statusCode, rw.metadata)
		}
	}
	return nil
}

func (rw *responseWriter) WriteHeader(statusCode int, metadata string) {
	rw.statusCode, rw.metadata = statusCode, metadata
}

func (rw *responseWriter) Write(p []byte) (int, error) {
	if err := rw.writeHeader(); err != nil {
		return 0, err
	}
	return rw.w.Write(p)
}

// Logger provides a simple interface for the Server to log to.
type Logger interface {
	Printf(format string, v ...any)
}

// Server defines parameters for running a Gemini server.
//
// The zero value for Server is not a valid configuration.
// The TLSConfig must be set and must contain at least one certificate
// or non-nil GetCertificate.
type Server struct {
	// Addr is the address to listen on.
	// Defaults to :1965 if empty.
	Addr string

	// Handler is invoked to handle all requests.
	Handler Handler

	// Logger logs various diagnostics if it is not nil.
	Logger Logger

	// TLSConfig configures the TLS.
	TLSConfig *tls.Config

	// ReadTimeout sets the maximum duration for reading an incoming request.
	ReadTimeout time.Duration

	// WriteTimeout sets the maximum duration before
	// timing out on writing an outgoing response.
	WriteTimeout time.Duration

	// Insecure disables TLS.
	// It should only be set if the server is behind a reverse proxy.
	// Insecure servers do not support Server Name Indication (SNI).
	Insecure bool
}

func (srv *Server) logf(format string, v ...any) {
	if srv.Logger != nil {
		srv.Logger.Printf(format, v...)
	}
}

// ListenAndServe starts the server loop.
// The server loop ends when the passed context is cancelled.
func (srv *Server) ListenAndServe(ctx context.Context) error {
	addr := srv.Addr
	if addr == "" {
		addr = ":1965"
	}

	l, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	defer l.Close()

	return srv.Serve(ctx, l)
}

// Serve starts the server loop and listens on a custom listener.
// The server loop ends when the passed context is cancelled.
func (srv *Server) Serve(ctx context.Context, l net.Listener) error {
	if !srv.Insecure {
		if srv.TLSConfig == nil {
			return errors.New("gemproto: nil Server.TLSConfig")
		} else if len(srv.TLSConfig.Certificates) == 0 && srv.TLSConfig.GetCertificate == nil {
			return errors.New("gemproto: no Server.TLSConfig certificates")
		}

		l = tls.NewListener(l, srv.TLSConfig)
	}

	var closed int32

	go func() {
		<-ctx.Done()
		atomic.StoreInt32(&closed, 1)
		l.Close()
	}()

	const maxBackoff = 1 * time.Second
	const defBackoff = 5 * time.Millisecond
	backoff := defBackoff

	for {
		conn, err := l.Accept()

		if err != nil {
			var ne net.Error
			if errors.As(err, &ne) && ne.Timeout() {
				srv.logf("gemproto: accept timeout: %v; retrying in %v", err, backoff)
				time.Sleep(backoff)
				backoff *= 2
				if backoff > maxBackoff {
					backoff = maxBackoff
				}
				continue
			}

			if atomic.LoadInt32(&closed) == 1 {
				return ErrServerClosed
			}

			srv.logf("gemproto: server listen error: %s", err)
			return err
		}

		backoff = defBackoff
		go srv.serve(ctx, conn)
	}
}

func (srv *Server) serve(ctx context.Context, conn net.Conn) {
	defer func() {
		if v := recover(); v != nil {
			srv.logf("gemproto: recover: %v", v)
		}
	}()

	defer conn.Close()

	now := time.Now()
	if srv.ReadTimeout > 0 {
		_ = conn.SetReadDeadline(now.Add(srv.ReadTimeout))
	}

	if srv.WriteTimeout > 0 {
		_ = conn.SetWriteDeadline(now.Add(srv.WriteTimeout))
	}

	if tlsConn, ok := conn.(*tls.Conn); ok {
		if err := tlsConn.HandshakeContext(ctx); err != nil {
			srv.logf("gemproto: tls handshake failed: %s", err)
			return
		}
	}

	if err := srv.respond(ctx, conn); err != nil {
		srv.logf("gemproto: error: %s", err)
	}
}

func (srv *Server) respond(ctx context.Context, conn net.Conn) error {
	rawURL, err := readHeaderLine(conn, 1026)
	if errors.Is(err, errHeaderLineTooLong) {
		return reply(conn, StatusBadRequest, "request line too long")
	} else if err != nil { // i/o error
		return err
	}

	var connState *tls.ConnectionState
	var serverName string

	if tlsConn, ok := conn.(*tls.Conn); ok {
		cs := tlsConn.ConnectionState()
		connState = &cs
		serverName = connState.ServerName
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return reply(conn, StatusBadRequest, "invalid url")
	}

	if u.Scheme == "" && u.Host == "" {
		u.Scheme = "gemini"
		u.Host = serverName
	}

	req := Request{
		URL:        u,
		RequestURI: rawURL,
		RemoteAddr: conn.RemoteAddr().String(),
		Host:       serverName,
		TLS:        connState,
		ctx:        ctx,
	}

	rw := responseWriter{
		w:          conn,
		statusCode: StatusOK,
		metadata:   gemtext.MIMEType,
	}

	defer func() { _ = rw.writeHeader() }()

	handler := srv.Handler
	if handler == nil {
		handler = NotFoundHandler()
	}

	handler.ServeGemini(&rw, &req)

	return nil
}

func reply(w io.Writer, code int, meta string) error {
	_, err := fmt.Fprint(w, code, " ", meta, "\r\n")
	return err
}
