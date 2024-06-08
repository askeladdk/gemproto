package gemproto_test

import (
	"crypto/x509/pkix"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/askeladdk/gemproto"
	"github.com/askeladdk/gemproto/gemcert"
	"github.com/askeladdk/gemproto/internal/require"
)

func TestHostsFile(t *testing.T) {
	t.Parallel()

	hf := gemproto.NewHostsFile(io.Discard)

	require.NoError(t, hf.SetHost(gemproto.Host{
		Addr:        "localhost:1965",
		Algorithm:   "sha256",
		Fingerprint: "1",
	}))

	h, _ := hf.Host("localhost:1965")
	h.Fingerprint += "1"
	err := hf.SetHost(h)
	require.NoError(t, err)
}

func TestHostsFileTOFU(t *testing.T) {
	hf := gemproto.NewHostsFile(io.Discard)

	t.Run("create", func(t *testing.T) {
		_, exists := hf.Host("localhost")
		require.True(t, !exists)

		cert, err := gemcert.CreateX509KeyPair(gemcert.CreateOptions{
			DNSNames: []string{"localhost"},
			Subject: pkix.Name{
				CommonName: "localhost",
			},
		})
		require.NoError(t, err)

		require.NoError(t, hf.TrustCertificate(cert.Leaf, "localhost"))

		_, exists = hf.Host("localhost")
		require.True(t, exists)

		require.NoError(t, hf.TrustCertificate(cert.Leaf, "localhost"))
	})

	t.Run("renew", func(t *testing.T) {
		renew, err := gemcert.CreateX509KeyPair(gemcert.CreateOptions{
			Subject: pkix.Name{
				CommonName: "localhost",
			},
			Duration: 1 * time.Hour,
		})
		require.NoError(t, err)
		require.NoError(t, hf.TrustCertificate(renew.Leaf, "localhost"))
	})

	t.Run("not trusted", func(t *testing.T) {
		fail, err := gemcert.CreateX509KeyPair(gemcert.CreateOptions{
			Subject: pkix.Name{
				CommonName: "localhost",
			},
		})
		require.NoError(t, err)
		require.ErrorIs(t, hf.TrustCertificate(fail.Leaf, "localhost"), gemproto.ErrCertificateNotTrusted)
	})
}

func TestHostsFileReadFrom(t *testing.T) {
	t.Parallel()

	hf := gemproto.NewHostsFile(io.Discard)
	_, err := hf.ReadFrom(strings.NewReader(`localhost sha256 abcdef 2050-12-31T00:00:00Z`))
	require.NoError(t, err)
	h, exists := hf.Host("localhost")
	require.True(t, exists)
	expected := gemproto.Host{
		Addr:        "localhost",
		Algorithm:   "sha256",
		Fingerprint: "abcdef",
		NotAfter:    time.Date(2050, 12, 31, 0, 0, 0, 0, time.UTC),
	}
	require.Equal(t, expected, h)
}

func TestOpenHostsFile(t *testing.T) {
	t.Parallel()

	tmpdir := os.TempDir()
	_, f, err := gemproto.OpenHostsFile(tmpdir + "/tmphosts")
	require.NoError(t, err)
	require.NoError(t, f.Close())
}
