// Package gemtext contains utilities for handling gemtext files.
package gemtext

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"unicode/utf8"
	"unsafe"
)

// MIMEType is the gemtext mimetype.
const MIMEType = "text/gemini;charset=utf-8"

func init() {
	_ = mime.AddExtensionType(".gmi", MIMEType)
	_ = mime.AddExtensionType(".gemini", MIMEType)
}

// Builder is used to efficiently build a gemtext file using the provided methods.
type Builder struct {
	b *bytes.Buffer
}

// NewBuilder returns a new Builder.
func NewBuilder(buf []byte) *Builder {
	return &Builder{
		b: bytes.NewBuffer(buf),
	}
}

// Bytes returns the accumulated bytes.
func (b *Builder) Bytes() []byte {
	return b.b.Bytes()
}

// String returns the accumulated string.
func (b *Builder) String() string {
	return zcstring(b.Bytes())
}

// Reset resets the builder to empty but retains the underlying storage.
func (b *Builder) Reset() {
	b.b.Reset()
}

// WriteTo writes the accumulated gemtext to w.
func (b *Builder) WriteTo(w io.Writer) (int64, error) {
	return b.b.WriteTo(w)
}

// Heading writes a '#' heading.
func (b *Builder) Heading(text string) {
	fmt.Fprintf(b.b, "# %s\n", text)
}

// SubHeading writes a '##' heading.
func (b *Builder) SubHeading(text string) {
	fmt.Fprintf(b.b, "## %s\n", text)
}

// SubSubHeading writes a '###' heading.
func (b *Builder) SubSubHeading(text string) {
	fmt.Fprintf(b.b, "### %s\n", text)
}

// Point writes a '*' list bullet point.
func (b *Builder) Point(text string) {
	fmt.Fprintf(b.b, "* %s\n", text)
}

// Pre toggles a preformatted block.
func (b *Builder) Pre(alt string) {
	fmt.Fprintf(b.b, "```%s\n", alt)
}

// Paragraph writes a paragraph of plain text.
func (b *Builder) Paragraph(text string) {
	fmt.Fprintf(b.b, "%s\n", text)
}

// Newline writes a newline.
func (b *Builder) Newline() {
	b.b.WriteByte('\n')
}

// Quote writes a '>' quote block.
// Text may contain multiple lines delimited by newlines.
// Each line is quoted on a separate line in the output.
func (b *Builder) Quote(text string) {
	if text == "" {
		b.Newline()
		return
	}

	for text != "" {
		var line string
		line, text, _ = strings.Cut(text, "\n")
		fmt.Fprintf(b.b, "> %s\n", line)
	}
}

// Link writes a '=>' link.
// The label is optional.
func (b *Builder) Link(url, label string) {
	if label == "" {
		fmt.Fprintf(b.b, "=> %s\n", url)
		return
	}

	fmt.Fprintf(b.b, "=> %s %s\n", url, label)
}

// TextAttachment attaches data by writing a query-escaped data URL link.
// The mimetype defaults to text/plain if it not provided.
func (b *Builder) TextAttachment(data, mimetype, label string) {
	if mimetype == "" {
		mimetype = "text/plain;charset=utf-8"
	}
	mimetype = strings.ReplaceAll(mimetype, " ", "")
	url := fmt.Sprintf("data:%s,%s", mimetype, url.QueryEscape(data))
	b.Link(url, label)
}

// BinaryAttachment attaches data by writing a base64-encoded data URL link.
// The mimetype defaults to application/octet-stream if it not provided.
func (b *Builder) BinaryAttachment(r io.Reader, mimetype, label string) {
	if mimetype == "" {
		mimetype = "application/octet-stream"
	}

	mimetype = strings.ReplaceAll(mimetype, " ", "")
	fmt.Fprintf(b.b, "=> data:%s;base64,", mimetype)
	enc := base64.NewEncoder(base64.StdEncoding, b.b)
	_, _ = io.Copy(enc, r)
	enc.Close()

	if label != "" {
		b.b.WriteByte(' ')
		b.b.WriteString(label)
	}
	b.Newline()
}

// Attachment attaches data by writing a data URL link.
// The data is either query-escaped or base64-encoded depending on its contents.
// The mimetype is detected from the data if it is not provided.
func (b *Builder) Attachment(data []byte, mimetype, label string) {
	if mimetype == "" {
		mimetype = http.DetectContentType(data)
	}

	if utf8.Valid(data) {
		b.TextAttachment(zcstring(data), mimetype, label)
		return
	}

	b.BinaryAttachment(bytes.NewReader(data), mimetype, label)
}

func zcstring(p []byte) string {
	return *(*string)(unsafe.Pointer(&p))
}
