// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This code is adapted from:
// https://cs.opensource.google/go/go/+/refs/tags/go1.19.2:src/net/http/fs.go

package gemproto

import (
	"bufio"
	"embed"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/askeladdk/gemproto/gemtext"
)

// Dir implements fs.FS for the local file system.
type Dir string

// Open implements fs.FS using os.Open, opening files for reading rooted
// and relative to the directory d.
func (d Dir) Open(name string) (fs.File, error) {
	if filepath.Separator != '/' && strings.ContainsRune(name, filepath.Separator) {
		return nil, errors.New("gemproto: invalid character in file path")
	}

	dir := string(d)
	if dir == "" {
		dir = "."
	}

	fullName := filepath.Join(dir, filepath.FromSlash(path.Clean("/"+name)))
	f, err := os.Open(fullName)
	if err != nil {
		return nil, mapOpenError(err, fullName, filepath.Separator, os.Stat)
	}
	return f, nil
}

// FileServerFlags enumerates all FileServer capability flags.
type FileServerFlags int

const (
	// ListDirs enables directory listing.
	ListDirs FileServerFlags = 1 << iota

	// ShowHiddenFiles enables dot-files to be listed.
	ShowHiddenFiles

	// UseMetaFile enables the .meta file to be scanned.
	UseMetaFile
)

type fileServer struct {
	Root  fs.FS
	Flags FileServerFlags
}

// FileServer returns a handler that serves Gemini requests
// with the contents of the file system rooted at root.
//
// As a special case, the returned file server redirects any request
// ending in "/index.gmi" to the same path, without the final
// "index.gmi".
//
// To use the operating system's file system implementation,
// use gemproto.Dir:
//
//	serveMux.Handle("/", gemproto.FileServer(gemproto.Dir("/tmp", 0)))
//
// # Flags
//
// FileServer accepts flags to enable certain capabilities.
//
// ListDirs enables listing directory contents if there is no index.gmi in the directory.
//
// ShowHiddenFiles enables hidden files and directories to be accessed.
//
// UseMetaFile enables parsing the .meta file to customize the metadata
// of any files accessed in the same directory as the .meta file.
//
// The .meta file has the following format:
//
// - Empty lines and lines starting with a '#' are ignored.
//
// - All other lines must have the form <pattern>:<metadata>,
// where <pattern> is a file pattern and metadata is either a mimetype
// or a valid Gemini response line.
// Mimetypes starting with ';' are appended.
// Response lines have the form <2digitcode><space><metadata>.
func FileServer(root fs.FS, flags FileServerFlags) Handler {
	return fileServer{
		Root:  root,
		Flags: flags,
	}
}

func (fsrv fileServer) ServeGemini(w ResponseWriter, r *Request) {
	upath := r.URL.Path
	// embed.FS does not work with leading /
	if _, isembed := fsrv.Root.(embed.FS); isembed {
		upath = strings.TrimPrefix(upath, "/")
	} else if !strings.HasPrefix(upath, "/") {
		upath = "/" + upath
		r.URL.Path = upath
	}
	fsrv.serveFile(w, r, fsrv.Root, path.Clean(upath), true)
}

func (fsrv fileServer) readMetadata(name string) string {
	base := path.Base(name)
	metafilepath := filepath.Join(path.Dir(name), ".meta")
	f, err := fsrv.Root.Open(metafilepath)
	if err != nil {
		return ""
	}
	defer f.Close()

	scan := bufio.NewScanner(f)
	for scan.Scan() {
		text := scan.Text()
		if len(text) == 0 || text[0] == '#' {
			continue
		}

		if pattern, meta, ok := strings.Cut(text, ":"); ok {
			if matched, _ := path.Match(strings.TrimSpace(pattern), base); matched {
				return strings.TrimSpace(meta)
			}
		}
	}

	return ""
}

var responseLineRE = regexp.MustCompile(`[0-9]{2} .+`)

func (fsrv fileServer) serveFile(w ResponseWriter, r *Request, fsys fs.FS, name string, redirect bool) {
	const indexPage = "/index.gmi"

	// redirect .../index.html to .../
	if strings.HasSuffix(r.URL.Path, indexPage) {
		path := strings.TrimSuffix(r.URL.Path, indexPage)
		if q := r.URL.RawQuery; q != "" {
			path += "?" + q
		}
		Redirect(w, r, path, StatusPermanentRedirect)
		return
	}

	// parse the .meta file
	var metadata string
	if fsrv.Flags&UseMetaFile != 0 {
		metadata = fsrv.readMetadata(name)

		if metadata != "" && responseLineRE.MatchString(metadata) {
			w.WriteHeader(0, "")
			fmt.Fprint(w, metadata, "\r\n")
			return
		}
	}

	f, err := fsys.Open(name)
	if err != nil {
		w.WriteHeader(StatusNotFound, err.Error())
		return
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		w.WriteHeader(StatusNotFound, err.Error())
		return
	}

	if fsrv.Flags&ShowHiddenFiles == 0 && strings.Contains(name, "/.") {
		w.WriteHeader(StatusNotFound, "Not Found")
		return
	}

	if redirect {
		// redirect to canonical path: / at end of directory url
		// r.URL.Path always begins with /
		url := r.URL.Path
		if fi.IsDir() {
			if url[len(url)-1] != '/' {
				Redirect(w, r, path.Base(url)+"/", StatusPermanentRedirect)
				return
			}
		} else {
			if url[len(url)-1] == '/' {
				Redirect(w, r, "../"+path.Base(url), StatusPermanentRedirect)
				return
			}
		}
	}

	if fi.IsDir() {
		// serve index page if it exists
		index := strings.TrimSuffix(name, "/") + indexPage
		if ff, err := fsys.Open(index); err == nil {
			defer ff.Close()
			serveContent(w, ff, index, "")
			return
		}

		if fsrv.Flags&ListDirs == 0 {
			w.WriteHeader(StatusNotFound, "Not Found")
			return
		}

		fsrv.serveDir(w, f, name)
		return
	}

	serveContent(w, f, name, metadata)
}

type anyDirs interface {
	sort.Interface
	Name(i int) string
	IsDir(i int) bool
	Size(i int) int64
}

type fileInfoDirs []fs.FileInfo

func (d fileInfoDirs) Size(i int) int64   { return d[i].Size() }
func (d fileInfoDirs) IsDir(i int) bool   { return d[i].IsDir() }
func (d fileInfoDirs) Name(i int) string  { return d[i].Name() }
func (d fileInfoDirs) Len() int           { return len(d) }
func (d fileInfoDirs) Swap(i, j int)      { d[i], d[j] = d[j], d[i] }
func (d fileInfoDirs) Less(i, j int) bool { return d[i].Name() < d[j].Name() }

type dirEntryDirs []fs.DirEntry

func (d dirEntryDirs) Size(i int) int64 {
	fi, err := d[i].Info()
	if err != nil {
		return 0
	}
	return fi.Size()
}

func (d dirEntryDirs) IsDir(i int) bool   { return d[i].IsDir() }
func (d dirEntryDirs) Name(i int) string  { return d[i].Name() }
func (d dirEntryDirs) Len() int           { return len(d) }
func (d dirEntryDirs) Swap(i, j int)      { d[i], d[j] = d[j], d[i] }
func (d dirEntryDirs) Less(i, j int) bool { return d[i].Name() < d[j].Name() }

func formatFileSize(size int64) (int64, string) {
	switch {
	case size >= 1<<30:
		return size / (1 << 30), "G"
	case size >= 1<<20:
		return size / (1 << 20), "M"
	case size >= 1<<10:
		return size / (1 << 10), "K"
	default:
		return size, "B"
	}
}

type readdirFS interface {
	Readdir(count int) ([]fs.FileInfo, error)
}

func (fsrv fileServer) serveDir(w ResponseWriter, f fs.File, name string) {
	var entries anyDirs
	var err error

	if rdf, ok := f.(fs.ReadDirFile); ok {
		var direntries dirEntryDirs
		direntries, err = rdf.ReadDir(-1)
		entries = direntries
	} else if rdf, ok := f.(readdirFS); ok {
		var fileinfoentries fileInfoDirs
		fileinfoentries, err = rdf.Readdir(-1)
		entries = fileinfoentries
	}

	if err != nil {
		w.WriteHeader(StatusTemporaryFailure, "Error reading directory")
		return
	}

	b := gemtext.NewBuilder(make([]byte, 0, 1024))

	if name == "/" {
		b.Heading(name)
	} else {
		b.Heading(name + "/")
	}

	if entries != nil {
		sort.Sort(entries)

		for i := 0; i < entries.Len(); i++ {
			filepath := entries.Name(i)
			if fsrv.Flags&ShowHiddenFiles == 0 && strings.HasPrefix(filepath, ".") {
				continue
			}

			if entries.IsDir(i) {
				filepath += "/"
			}

			fz, ft := formatFileSize(entries.Size(i))
			label := fmt.Sprintf("%s (%d%s)", filepath, fz, ft)
			b.Link(filepath, label)
		}
	}

	_, _ = w.Write(b.Bytes())
}

func serveContent(w ResponseWriter, f fs.File, name, mimetype string) {
	var toappend string
	if strings.HasPrefix(mimetype, ";") {
		toappend, mimetype = mimetype, ""
	}

	if mimetype == "" {
		mimetype = mime.TypeByExtension(filepath.Ext(name))
		if mimetype == "" {
			mimetype = "application/octet-stream"
		}
	}

	w.WriteHeader(StatusOK, mimetype+toappend)
	_, _ = io.Copy(w, f)
}

// mapOpenError maps the provided non-nil error from opening name
// to a possibly better non-nil error. In particular, it turns OS-specific errors
// about opening files in non-directories into fs.ErrNotExist. See Issues 18984 and 49552.
func mapOpenError(originalErr error, name string, sep rune, stat func(string) (fs.FileInfo, error)) error {
	if errors.Is(originalErr, fs.ErrNotExist) || errors.Is(originalErr, fs.ErrPermission) {
		return originalErr
	}

	parts := strings.Split(name, string(sep))
	for i := range parts {
		if parts[i] == "" {
			continue
		}
		fi, err := stat(strings.Join(parts[:i+1], string(sep)))
		if err != nil {
			return originalErr
		}
		if !fi.IsDir() {
			return fs.ErrNotExist
		}
	}
	return originalErr
}
