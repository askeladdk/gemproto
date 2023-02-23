package gemproto

import (
	"bufio"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/askeladdk/gemproto/gemcert"
)

var ErrCertificateNotTrusted = errors.New("gemproto: certificate not trusted")

// Host is an entry in HostsFile.
type Host struct {
	// Addr is the domain:port of the remote host.
	Addr string

	// Algorithm is the algorithm used to compute the fingerprint.
	Algorithm string

	// Fingerprint is a hash of the host's certificate.
	Fingerprint string

	// NotAfter is the expiry time of the certificate.
	NotAfter time.Time
}

// HostsFile implements the Trust-On-First-Use (TOFU) mechanism
// by maintaining a set of known hosts in an append-only hostsfile.
//
// HostsFile is intended to be used with Client by setting the TrustCertificate field.
// TrustCertificate applies the TOFU algorithm and updates the hostsfile as needed.
//
// The hostsfile is append-only but HostsFile only stores the latest entries in memory.
// Older entries are retained for auditing purposes.
//
// HostsFile is safe to use concurrently.
//
// # File Format
//
// Each line in the hostsfile is an entry.
// An entry consists of four fields separated by spaces and delimited by a newline:
//
// address<SPACE>algorithm<SPACE>fingerprint<SPACE>expiry<LF>
//
//   - address is the domain:port of the remote host.
//   - port is the port number of the remote host.
//   - algorithm is the hashing algorithm used to compute the fingerprint.
//   - fingerprint is the base64 encoding of the hash of the certificate's Subject Public Key Info (SPKI) section.
//   - expiry is the expiration date of the certificate.
//
// Later entries overwrite older entries.
// Lines that do not conform to this format are ignored.
type HostsFile struct {
	hosts map[string]Host
	w     io.Writer
	mu    sync.RWMutex
}

// NewHostsFile returns a new HostsFile.
//
// New entries are written to w and flushed if w implements `Flush() error`.
func NewHostsFile(w io.Writer) *HostsFile {
	return &HostsFile{
		hosts: make(map[string]Host),
		w:     w,
	}
}

// Host returns the Host associated with the domain:port address.
func (hf *HostsFile) Host(addr string) (h Host, exists bool) {
	hf.mu.RLock()
	defer hf.mu.RUnlock()
	h, ok := hf.hosts[addr]
	return h, ok
}

// SetHost sets the host entry and writes it to the Writer set by NewHostsFile.
func (hf *HostsFile) SetHost(h Host) error {
	hf.mu.Lock()
	defer hf.mu.Unlock()

	if h2, ok := hf.hosts[h.Addr]; ok && h == h2 {
		return nil
	}

	hf.hosts[h.Addr] = h

	notAfter := h.NotAfter.Format(time.RFC3339)
	if _, err := fmt.Fprintf(hf.w, "%s %s %s %s\n",
		h.Addr, h.Algorithm, h.Fingerprint, notAfter); err != nil {
		return err
	}

	if flusher, ok := hf.w.(interface{ Flush() error }); ok {
		return flusher.Flush()
	}

	return nil
}

// TrustCertificate applies the Trust On First Use algorithm
// to the given certificate and remote host address.
func (hf *HostsFile) TrustCertificate(cert *x509.Certificate, addr string) error {
	// implementation based on
	// gemini://makeworld.space/gemlog/2020-07-03-tofu-rec.gmi

	const algo = "sha256"
	notAfter := cert.NotAfter.UTC()
	fp := gemcert.Fingerprint(cert)

	if h, ok := hf.Host(addr); ok {
		// fingerprint mismatch
		if algo != h.Algorithm || fp != h.Fingerprint {
			// stored certificate has expired, renew it
			if time.Now().UTC().After(h.NotAfter) {
				goto renew
			}

			// fingerprint mismatch but cert not expired
			return ErrCertificateNotTrusted
		}

		// fingerprint and expiry matches
		if h.NotAfter.Equal(notAfter) {
			return nil
		}
	}

renew:
	host, _ := splitHostPort(addr)
	if err := verifyHostname(cert, host); err != nil {
		return err
	}

	return hf.SetHost(Host{
		Addr:        addr,
		Algorithm:   algo,
		Fingerprint: fp,
		NotAfter:    notAfter,
	})
}

// ReadFrom parses a hostsfile and stores the entries in memory.
// Later entries overwrite earlier ones.
func (hf *HostsFile) ReadFrom(r io.Reader) (n int64, err error) {
	hf.mu.Lock()
	defer hf.mu.Unlock()

	cr := countReader{r: r}
	sc := bufio.NewScanner(&cr)

	for sc.Scan() {
		text := sc.Text()

		// skip empty lines and lines starting with '#'.
		if len(text) == 0 || text[0] == '#' {
			continue
		}

		fields := strings.Fields(text)
		if len(fields) == 4 {
			if notAfter, err := time.Parse(time.RFC3339, fields[3]); err == nil {
				h := Host{
					Addr:        fields[0],
					Algorithm:   fields[1],
					Fingerprint: fields[2],
					NotAfter:    notAfter.UTC(),
				}
				hf.hosts[h.Addr] = h
			}
		}
	}

	return cr.n, sc.Err()
}

func verifyHostname(cert *x509.Certificate, hostname string) error {
	// check if Common Name is already in DNSNames
	for _, dnsName := range cert.DNSNames {
		if dnsName == cert.Subject.CommonName {
			return cert.VerifyHostname(hostname)
		}
	}

	// fixup cert to also verify legacy Common Name field
	temp := *cert
	temp.DNSNames = make([]string, 0, len(temp.DNSNames)+1)
	temp.DNSNames = append(temp.DNSNames, temp.DNSNames...)
	temp.DNSNames = append(temp.DNSNames, temp.Subject.CommonName)
	return temp.VerifyHostname(hostname)
}

// OpenHostsFile is a shorthand for opening and reading a hostsfile.
// The file is opened in append mode and is created if it does not exist yet.
// The callee is responsible for calling os/File.Close to close the file.
func OpenHostsFile(name string) (*HostsFile, *os.File, error) {
	f, err := os.OpenFile(name, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, nil, err
	}
	hf := NewHostsFile(f)
	if _, err := hf.ReadFrom(f); err != nil {
		defer f.Close()
		return nil, nil, err
	}
	return hf, f, nil
}

type countReader struct {
	r io.Reader
	n int64
}

func (r *countReader) Read(p []byte) (int, error) {
	n, err := r.r.Read(p)
	r.n += int64(n)
	return n, err
}
