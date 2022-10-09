package gemproto

import (
	"errors"
	"io"
	"net"
	urlpkg "net/url"
	"path"
	"strings"
)

var errHeaderLineTooLong = errors.New("gemproto: header line too long")

func readHeaderLine(r io.Reader, maxlen int) (string, error) {
	var buf [2048]byte

	for i := 0; i < maxlen; i++ {
		if _, err := r.Read(buf[i : i+1]); err != nil {
			return "", err
		}

		if i > 0 && buf[i-1] == '\r' && buf[i] == '\n' {
			return string(buf[:i-1]), nil
		}
	}

	return "", errHeaderLineTooLong
}

// absoluteURL makes the url path absolute by combining
// with request path, if the url is relative.
func absoluteURL(r *Request, url string) string {
	if u, err := urlpkg.Parse(url); err == nil {
		if u.Scheme == "" && u.Host == "" {
			u.Scheme = r.URL.Scheme
			u.Host = r.URL.Host

			oldpath := r.URL.Path
			if oldpath == "" { // should not happen, but avoid a crash if it does
				oldpath = "/"
			}

			if url == "" || url[0] != '/' {
				trailing := strings.HasSuffix(url, "/")
				// make relative path absolute
				olddir, _ := path.Split(oldpath)
				u.Path = path.Clean(olddir + url)

				if trailing && !strings.HasSuffix(u.Path, "/") {
					u.Path += "/"
				}
			}

			url = u.String()
		}
	}

	return url
}

// splitHostPort splits the host and port.
// If there is no port, only the host is returned.
func splitHostPort(addr string) (host, port string) {
	if !strings.Contains(addr, ":") {
		return addr, ""
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return addr, ""
	}
	return host, port
}
