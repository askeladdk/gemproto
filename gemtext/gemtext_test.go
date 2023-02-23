package gemtext

import (
	"io"
	"reflect"
	"strings"
	"testing"
)

func assertEqual(t *testing.T, a, b any) {
	if !reflect.DeepEqual(a, b) {
		t.Error(a, "is not", b)
	}
}

func TestBuilder(t *testing.T) {
	b := NewBuilder(nil)
	b.Attachment([]byte("hello world"), "", "hello.txt")
	assertEqual(t, b.String(), "=> data:text/plain;charset=utf-8,hello+world hello.txt\n")
	b.Reset()
	b.TextAttachment("hello world", "", "")
	assertEqual(t, b.String(), "=> data:text/plain;charset=utf-8,hello+world\n")
	b.Reset()
	b.BinaryAttachment(strings.NewReader("hello world"), "", "hello.txt")
	assertEqual(t, b.String(), "=> data:application/octet-stream;base64,aGVsbG8gd29ybGQ= hello.txt\n")
	b.Reset()
	b.Heading("First Heading")
	b.SubHeading("Second Heading")
	b.SubSubHeading("Third Heading")
	assertEqual(t, b.String(), "# First Heading\n## Second Heading\n### Third Heading\n")
	b.Reset()
	b.Point("(1) bullet")
	b.Point("(2) bullet")
	b.Point("(3) bullet")
	assertEqual(t, b.String(), "* (1) bullet\n* (2) bullet\n* (3) bullet\n")
	b.Reset()
	b.Pre("code sample")
	b.Paragraph(`print("hello world")`)
	b.Pre("")
	assertEqual(t, b.String(), "```code sample\nprint(\"hello world\")\n```\n")
	b.Reset()
	b.Quote("Tempore et quasi dolorum et.\nCorporis quis ut consectetur.\nAliquam omnis id aperiam ut fuga pariatur fugit aliquam")
	b.Newline()
	assertEqual(t, b.String(), "> Tempore et quasi dolorum et.\n> Corporis quis ut consectetur.\n> Aliquam omnis id aperiam ut fuga pariatur fugit aliquam\n\n")
	b.Reset()
	b.Quote("")
	assertEqual(t, b.String(), "\n")
	b.Reset()
	b.Link("gemini://localhost", "")
	assertEqual(t, b.String(), "=> gemini://localhost\n")
	b.Reset()
	_, _ = b.WriteTo(io.Discard)
}
