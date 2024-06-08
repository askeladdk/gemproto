// Copyright (c) 2020 Adnan Maolood
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

// This code is adapted from:
// https://git.sr.ht/~adnano/go-gemini/tree/master/item/certificate/create.go

// Package gemcert provides utilities for creating certificates.
package gemcert

import (
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"io"
	"math/big"
	"net"
	"os"
	"time"
)

// CreateOptions configures the creation of a TLS certificate
// generated with the Ed25519 signature algorithm.
type CreateOptions struct {
	// DNSNames should contain the DNS names that this certificate is valid for.
	DNSNames []string

	// IPAdresses Should contain the IP addresses that the certificate is valid for.
	IPAddresses []net.IP

	// Subject specifies the certificate Subject.
	//
	// Subject.CommonName can contain the DNS name that this certificate
	// is valid for. Server certificates should specify both a Subject
	// and a Subject Alternate Name.
	Subject pkix.Name

	// Duration specifies the amount of time that the certificate is valid for.
	Duration time.Duration

	// Rand sets the random number generator.
	// If nil, crypto/rand.Reader is used.
	Rand io.Reader

	// Parent is the optional certificate to sign with.
	// If nil, the certificate will be self-signed.
	Parent *x509.Certificate
}

func newX509KeyPair(options CreateOptions) (*x509.Certificate, crypto.PrivateKey, error) {
	randr := rand.Reader
	if options.Rand != nil {
		randr = options.Rand
	}

	var pub crypto.PublicKey
	var priv crypto.PrivateKey
	var err error
	pub, priv, err = ed25519.GenerateKey(randr)
	if err != nil {
		return nil, nil, err
	}

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(randr, serialNumberLimit)
	if err != nil {
		return nil, nil, err
	}

	notBefore := time.Now()
	notAfter := notBefore.Add(options.Duration)

	template := x509.Certificate{
		SerialNumber:          serialNumber,
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IPAddresses:           options.IPAddresses,
		DNSNames:              options.DNSNames,
		Subject:               options.Subject,
	}

	parent := options.Parent
	if parent == nil {
		parent = &template
	}

	crt, err := x509.CreateCertificate(randr, &template, parent, pub, priv)
	if err != nil {
		return nil, nil, err
	}

	cert, err := x509.ParseCertificate(crt)
	if err != nil {
		return nil, nil, err
	}

	return cert, priv, nil
}

// CreateX509KeyPair creates a new TLS certificate.
func CreateX509KeyPair(options CreateOptions) (tls.Certificate, error) {
	crt, priv, err := newX509KeyPair(options)
	if err != nil {
		return tls.Certificate{}, err
	}
	var cert tls.Certificate
	cert.Leaf = crt
	cert.Certificate = append(cert.Certificate, crt.Raw)
	cert.PrivateKey = priv
	return cert, nil
}

// StoreX509KeyPair stores the public and private keys of
// the provided certificate in their respective files.
func StoreX509KeyPair(cert tls.Certificate, certFile, keyFile string) error {
	certOut, err := os.OpenFile(certFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer certOut.Close()

	keyOut, err := os.OpenFile(keyFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer keyOut.Close()

	if err := pem.Encode(certOut, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cert.Leaf.Raw,
	}); err != nil {
		return err
	}

	privBytes, err := x509.MarshalPKCS8PrivateKey(cert.PrivateKey)
	if err != nil {
		return err
	}

	return pem.Encode(keyOut, &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privBytes,
	})
}

// LoadX509KeyPair reads and parses a public/private key pair from a pair of files.
// The files must be PEM encoded.
// Certificate.Leaf will contain the parsed form of the certificate.
func LoadX509KeyPair(certFile, keyFile string) (cert tls.Certificate, err error) {
	if cert, err = tls.LoadX509KeyPair(certFile, keyFile); err != nil {
		return cert, err
	}

	if cert.Leaf == nil {
		cert.Leaf, err = x509.ParseCertificate(cert.Certificate[0])
	}

	return cert, err
}

// Fingerprint returns the hexadecimal encoding of the sha256 hash
// of the given certificate's Subject Public Key Info (SPKI) section.
func Fingerprint(cert *x509.Certificate) string {
	h := sha256.Sum256(cert.RawSubjectPublicKeyInfo)
	return hex.EncodeToString(h[:])
}
