package gemproto

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"
)

// ErrInvalidResponse is returned by Client if it received an invalid response.
var ErrInvalidResponse = errors.New("gemproto: invalid response")

// RedirectError is returned by Client.Do if the
// maximum number of redirects has been exceeded.
type RedirectError struct {
	// LastURL is the last URL that the client was redirected from.
	LastURL string

	// NextURL is the next URL that the client was redirected to.
	NextURL string
}

// Error implements the error interface.
func (err RedirectError) Error() string {
	return fmt.Sprintf("gemproto: too many redirects: %s", err.NextURL)
}

type nopReader struct{}

func (*nopReader) Read([]byte) (int, error) { return 0, io.EOF }

var nopReadCloser io.ReadCloser = io.NopCloser((*nopReader)(nil))

type dialer struct {
	*tls.Dialer
	hostsFile  *HostsFile
	serverAddr string
}

func (d *dialer) verifyConnection(cs tls.ConnectionState) error {
	if d.hostsFile != nil {
		return d.hostsFile.TrustCertificate(cs.PeerCertificates[0], d.serverAddr)
	}
	return nil
}

// GetCertificateFunc is a function that maps a hostname to a certificate.
type GetCertificateFunc func(hostname string) (tls.Certificate, bool)

// SingleClientCertificate returns the same certificate regardless of hostname.
func SingleClientCertificate(cert tls.Certificate) GetCertificateFunc {
	return func(string) (tls.Certificate, bool) {
		return cert, true
	}
}

// Client implements the client side of the Gemini protocol.
//
// The client must close the response body when done with it:
//
//	client := gemproto.Client{}
//	resp, err := client.Get("gemini://gemini.circumlunar.space")
//	if err != nil {
//	  // handle error
//	}
//	defer resp.Body.Close()
//	body, err := io.ReadAll(resp.Body)
//	// ...
//
// Client can optionally provide a certificate to authenticate itself to hosts:
//
//	cert, err := gemcert.LoadX509KeyPair("client.crt", "client.key")
//	if err != nil {
//	  // handle error
//	}
//	client := gemproto.Client{
//	  GetCertificate: gemproto.SingleClientCertificate(cert),
//	}
//	// ...
//
// Client can optionally verify and record host certificates
// using the TOFU mechanism by creating a HostsFile:
//
//	hostsfile, f, err := gemproto.OpenHostsFile("./hostsfile")
//	if err != nil {
//	  // handle error
//	}
//	defer f.Close()
//	client := gemproto.Client{
//	  HostsFile: hostsfile,
//	}
//	// ...
type Client struct {
	// ConnectTimeout sets the idle timeout.
	ConnectTimeout time.Duration

	// ReadTimeout sets the read timeout.
	ReadTimeout time.Duration

	// WriteTimeout sets the write timeout.
	WriteTimeout time.Duration

	// HostsFile is optional and specifies to verify hosts.
	HostsFile *HostsFile

	// GetCertificate is optional and maps hostnames to client certificates.
	GetCertificate GetCertificateFunc
}

// Get issues a request to the specified URL.
func (c *Client) Get(rawURL string) (*Response, error) {
	req, err := NewRequestWithContext(context.Background(), rawURL)
	if err != nil {
		return nil, err
	}
	return c.Do(req)
}

// Do sends a request and returns a response.
func (c *Client) Do(req *Request) (*Response, error) {
	const maxRedirects = 5

	if req.URL == nil {
		return nil, errors.New("gemproto: nil Request.URL")
	} else if req.URL.Scheme != "gemini" {
		return nil, errors.New("gemproto: Request.URL.Scheme is not gemini")
	}

	d := dialer{
		Dialer: &tls.Dialer{
			NetDialer: &net.Dialer{
				Timeout: c.ConnectTimeout,
			},
			Config: &tls.Config{
				MinVersion:         tls.VersionTLS12,
				InsecureSkipVerify: true,
			},
		},
		hostsFile: c.HostsFile,
	}

	d.Dialer.Config.VerifyConnection = d.verifyConnection

	return c.do(req, &d, maxRedirects)
}

func (c *Client) do(r *Request, d *dialer, redirects int) (*Response, error) {
	host, port := splitHostPort(r.Host)

	if host == "" {
		host, port = r.URL.Hostname(), r.URL.Port()
	}

	if port == "" {
		port = "1965"
	}

	if c.GetCertificate != nil && host != d.Config.ServerName {
		if cert, ok := c.GetCertificate(host); ok {
			d.Config.Certificates = []tls.Certificate{cert}
		} else {
			d.Config.Certificates = nil
		}
	}

	addr := net.JoinHostPort(host, port)

	d.Config.ServerName = host
	d.serverAddr = addr

	conn, err := d.DialContext(r.Context(), "tcp", addr)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	if c.ReadTimeout > 0 {
		_ = conn.SetReadDeadline(now.Add(c.ReadTimeout))
	}
	if c.WriteTimeout > 0 {
		_ = conn.SetWriteDeadline(now.Add(c.WriteTimeout))
	}

	status, meta, err := c.doReqRes(conn, r.URL.String())
	if err != nil {
		defer conn.Close()
		return nil, err
	}

	// handle redirects
	if status[0] == '3' {
		defer conn.Close()

		if redirects == 0 {
			return nil, RedirectError{
				LastURL: r.URL.String(),
				NextURL: meta,
			}
		}

		newreq, err := NewRequestWithContext(r.Context(), absoluteURL(r, meta))
		if err != nil {
			return nil, err
		}

		return c.do(newreq, d, redirects-1)
	}

	statusCode, _ := strconv.Atoi(status)

	connState := conn.(*tls.Conn).ConnectionState()

	var body io.ReadCloser = conn

	// only 2x responses have a body
	if status[0] != '2' {
		defer conn.Close()
		body = nopReadCloser
	}

	return &Response{
		StatusCode: statusCode,
		Meta:       meta,
		Body:       body,
		TLS:        &connState,
	}, nil
}

func (c *Client) doReqRes(conn net.Conn, rawURL string) (status, meta string, err error) {
	if _, err = fmt.Fprintf(conn, "%s\r\n", rawURL); err != nil {
		return
	}

	var line string
	if line, err = readHeaderLine(conn, 1029); err != nil {
		return
	}

	// status is required but meta is optional
	if status, meta, _ = strings.Cut(line, " "); len(status) == 0 {
		err = ErrInvalidResponse
	}

	return
}
