package gemproto

import (
	"strings"
	"testing"
)

func TestReadHeaderLine(t *testing.T) {
	t.Parallel()

	for _, testcase := range []struct {
		Name     string
		Line     string
		Expected string
		Err      bool
	}{
		{
			Name: "empty",
			Line: "",
			Err:  true,
		},
		{
			Name:     "only newline",
			Line:     "\r\n",
			Expected: "",
		},
		{
			Name:     "normal",
			Line:     "a.b.c\r\n",
			Expected: "a.b.c",
		},
		{
			Name: "max spaces no newline",
			Line: strings.Repeat(" ", 1029),
			Err:  true,
		},
		{
			Name: "almost max spaces newline",
			Line: strings.Repeat(" ", 1028) + "\r\n",
			Err:  true,
		},
		{
			Name:     "spaces newline",
			Line:     strings.Repeat(" ", 1027) + "\r\n",
			Expected: strings.Repeat(" ", 1027),
		},
	} {
		t.Run(testcase.Name, func(t *testing.T) {
			line, err := readHeaderLine(strings.NewReader(testcase.Line), 1029)
			if (err != nil) != testcase.Err {
				t.Error(err)
			}
			if line != testcase.Expected {
				t.Error(line)
			}
		})
	}
}
