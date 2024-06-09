package main

import (
	"context"
	"crypto/tls"
	"crypto/x509/pkix"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/askeladdk/gemproto"
	"github.com/askeladdk/gemproto/gemcert"
)

func die(err error) {
	fmt.Println(err)
	os.Exit(1)
}

func capsule(args []string) {
	fset := flag.NewFlagSet("capsule", flag.ExitOnError)

	var (
		addr     = fset.String("addr", "0.0.0.0:1965", "host:port to listen on")
		certfile = fset.String("certfile", "server.crt", "public key")
		keyfile  = fset.String("keyfile", "server.key", "private key")
	)

	if err := fset.Parse(args); err != nil {
		fmt.Println(err)
		fset.Usage()
		return
	}

	dir := fset.Arg(0)
	dir, _ = filepath.Abs(dir)

	cert, err := tls.LoadX509KeyPair(*certfile, *keyfile)
	if err != nil {
		fmt.Println("error when loading key pair:", err)
		fset.Usage()
		return
	}

	mux := gemproto.NewServeMux()
	mux.Mount("/", gemproto.FileServer(gemproto.Dir(dir),
		gemproto.UseMetaFile|gemproto.ListDirs))

	srv := gemproto.Server{
		Addr:    *addr,
		Handler: mux,
		Logger:  log.Default(),
		TLSConfig: &tls.Config{
			MinVersion:   tls.VersionTLS12,
			ClientAuth:   tls.RequestClientCert,
			Certificates: []tls.Certificate{cert},
		},
	}

	log.Default().SetFlags(log.LstdFlags | log.LUTC)
	log.Printf("listening on %s\n", srv.Addr)

	ctx := context.Background()
	if err := srv.ListenAndServe(ctx); !errors.Is(err, gemproto.ErrServerClosed) {
		log.Println(err)
	}
}

func get(args []string) {
	fset := flag.NewFlagSet("get", flag.ExitOnError)

	var (
		certfile = fset.String("certfile", "", "public key")
		keyfile  = fset.String("keyfile", "", "private key")
	)

	if err := fset.Parse(args); err != nil {
		fset.Usage()
		die(err)
	}

	rawURL := fset.Arg(0)

	client := gemproto.Client{
		ConnectTimeout: 1 * time.Second,
		WriteTimeout:   10 * time.Second,
		ReadTimeout:    600 * time.Second,
	}

	if *certfile != "" && *keyfile != "" {
		cert, err := tls.LoadX509KeyPair(*certfile, *keyfile)
		if err != nil {
			die(err)
		}

		client.GetCertificate = gemproto.SingleClientCertificate(cert)
	}

	res, err := client.Get(rawURL)
	if err != nil {
		die(err)
	}
	defer res.Body.Close()

	if _, err := io.Copy(os.Stdout, res.Body); err != nil {
		die(err)
	}
}

func makecert(args []string) {
	fset := flag.NewFlagSet("makecert", flag.ExitOnError)

	var (
		crtout = fset.String("out", "", "public key")
		keyout = fset.String("keyout", "", "private key")
		name   = fset.String("name", "", "common name")
		days   = fset.Int("days", 365, "days the cert is valid for")
	)

	if err := fset.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		fset.Usage()
	}

	if *crtout == "" || *keyout == "" || *name == "" || *days == 0 {
		fset.Usage()
		os.Exit(1)
	}

	cert, err := gemcert.CreateX509KeyPair(gemcert.CreateOptions{
		Duration: time.Duration(*days) * 24 * time.Hour,
		DNSNames: []string{*name},
		Subject: pkix.Name{
			CommonName: *name,
		},
	})
	if err != nil {
		die(err)
	}

	if err := gemcert.StoreX509KeyPair(cert, *crtout, *keyout); err != nil {
		die(err)
	}
}

func viewcert(args []string) {
	fset := flag.NewFlagSet("viewcert", flag.ExitOnError)

	var (
		certfile = fset.String("certfile", "", "public key")
		keyfile  = fset.String("keyfile", "", "private key")
	)

	if err := fset.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		fset.Usage()
	}

	cert, err := gemcert.LoadX509KeyPair(*certfile, *keyfile)
	if err != nil {
		die(err)
	}

	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', 0)
	fmt.Fprintf(tw, "Subject\t%s\n", cert.Leaf.Subject.String())
	fmt.Fprintf(tw, "Issuer\t%s\n", cert.Leaf.Issuer.String())
	fmt.Fprintf(tw, "DNS Names\t%s\n", strings.Join(cert.Leaf.DNSNames, ", "))
	fmt.Fprintf(tw, "Not Before\t%s\n", cert.Leaf.NotBefore.Format(time.RFC1123))
	fmt.Fprintf(tw, "Not After\t%s\n", cert.Leaf.NotAfter.Format(time.RFC1123))
	fmt.Fprintf(tw, "Algorithm\t%s\n", cert.Leaf.PublicKeyAlgorithm)
	fmt.Fprintf(tw, "Fingerprint\t%s\n", gemcert.Fingerprint(cert.Leaf))
	tw.Flush()
}

func main() {
	var command string

	if len(os.Args) >= 2 {
		command = os.Args[1]
	}

	switch command {
	case "capsule":
		capsule(os.Args[2:])
	case "get":
		get(os.Args[2:])
	case "makecert":
		makecert(os.Args[2:])
	case "viewcert":
		viewcert(os.Args[2:])
	default:
		fmt.Println("Usage of gemini:")
		fmt.Println("  gemini capsule [-addr=:1965] [-certfile=server.crt] [-keyfile=server.key] root")
		fmt.Println("    Launch a capsule into Geminispace.")
		fmt.Println("  gemini get [-certfile=<path>] [-keyfile=<path>] <uri>")
		fmt.Println("    Retrieve and stream a Gemini resource to stdout.")
		fmt.Println("  gemini makecert -out=<path> -name=<name> -days=<n>")
		fmt.Println("    Generate a fresh self-signed certificate.")
		fmt.Println("  gemini viewcert -certfile=<path> -keyfile=<path>")
		fmt.Println("    View certificate details.")
	}
}
