package gemtext

import (
	"io"
	"strings"
	"testing"

	"github.com/askeladdk/gemproto/internal/require"
)

func TestBuilder(t *testing.T) {
	b := NewBuilder(nil)
	b.Attachment([]byte("hello world"), "", "hello.txt")
	require.Equal(t, b.String(), "=> data:text/plain;charset=utf-8,hello+world hello.txt\n")
	b.Reset()
	b.TextAttachment("hello world", "", "")
	require.Equal(t, b.String(), "=> data:text/plain;charset=utf-8,hello+world\n")
	b.Reset()
	b.BinaryAttachment(strings.NewReader("hello world"), "", "hello.txt")
	require.Equal(t, b.String(), "=> data:application/octet-stream;base64,aGVsbG8gd29ybGQ= hello.txt\n")
	b.Reset()
	b.Heading("First Heading")
	b.SubHeading("Second Heading")
	b.SubSubHeading("Third Heading")
	require.Equal(t, b.String(), "# First Heading\n## Second Heading\n### Third Heading\n")
	b.Reset()
	b.Point("(1) bullet")
	b.Point("(2) bullet")
	b.Point("(3) bullet")
	require.Equal(t, b.String(), "* (1) bullet\n* (2) bullet\n* (3) bullet\n")
	b.Reset()
	b.Pre("code sample")
	b.Paragraph(`print("hello world")`)
	b.Pre("")
	require.Equal(t, b.String(), "```code sample\nprint(\"hello world\")\n```\n")
	b.Reset()
	b.Quote("Tempore et quasi dolorum et.\nCorporis quis ut consectetur.\nAliquam omnis id aperiam ut fuga pariatur fugit aliquam")
	b.Newline()
	require.Equal(t, b.String(), "> Tempore et quasi dolorum et.\n> Corporis quis ut consectetur.\n> Aliquam omnis id aperiam ut fuga pariatur fugit aliquam\n\n")
	b.Reset()
	b.Quote("")
	require.Equal(t, b.String(), "\n")
	b.Reset()
	b.Link("gemini://localhost", "")
	require.Equal(t, b.String(), "=> gemini://localhost\n")
	b.Reset()
	_, _ = b.WriteTo(io.Discard)
}
