package dockerfile

import (
	"fmt"
	"strings"
	"testing"

	"github.com/openshift/imagebuilder/dockerfile/command"
)

// TestKeyValueInstructions tests calling derivatives of keyValueInstruction
// with multiple inputs.
func TestKeyValueInstructions(t *testing.T) {
	keyValuesInstructions := []struct {
		f   func([]KeyValue) (string, error)
		cmd string
	}{
		{Env, command.Env},
		{Label, command.Label},
	}
	testCases := []struct {
		in   []KeyValue
		want string
	}{
		{
			in:   nil,
			want: ``,
		},
		{
			in:   []KeyValue{},
			want: ``,
		},
		{
			in: []KeyValue{
				{"", ""},
				{"", "ABC"},
				{"ABC", ""},
			},
			want: `""="" ""="ABC" "ABC"=""`,
		},
		{
			in: []KeyValue{
				{"GOPATH", "/go"},
				{"MSG", "Hello World!"},
			},
			want: `"GOPATH"="/go" "MSG"="Hello World!"`,
		},
		{
			in: []KeyValue{
				{"PATH", "/bin"},
				{"GOPATH", "/go"},
				{"PATH", "$GOPATH/bin:$PATH"},
			},
			want: `"PATH"="/bin" "GOPATH"="/go" "PATH"="$GOPATH/bin:$PATH"`,
		},
		{
			in: []KeyValue{
				{"你好", "我会说汉语"},
			},
			want: `"你好"="我会说汉语"`,
		},
		{
			// This tests handling an string encoding edge case.
			// Example input taken from Docker parser's test suite.
			in: []KeyValue{
				{"☃", "'\" \\ / \b \f \n \r \t \x00"},
			},
			want: `"☃"="'\" \\ / \b \f \n \r \t \x00"`,
		},
		{
			// We should verify that HTML symbols < > & are not escaped,
			// as it is perfectly fine and expected for them to be used
			// in Dockerfile
			in: []KeyValue{
				{"URL", "https://domain.name/key1=val1&key2=val2"},
				{"NAME", "Person Name <person.name@domain.name>"},
			},
			want: `"URL"="https://domain.name/key1=val1&key2=val2"` +
				` "NAME"="Person Name <person.name@domain.name>"`,
		},
	}
	for _, tc := range testCases {
		for _, kvi := range keyValuesInstructions {
			got, err := kvi.f(tc.in)
			if err != nil {
				t.Fatal(err)
			}
			want := strings.TrimRight(fmt.Sprintf("%s %s", strings.ToUpper(kvi.cmd), tc.want), " ")
			if got != want {
				t.Errorf("%s(%v) = %q; want %q", strings.Title(kvi.cmd), tc.in, got, want)
			}
		}
	}
}

// TestFrom tests calling From with multiple inputs.
func TestFrom(t *testing.T) {
	testCases := []struct {
		in   string
		want string
	}{
		{
			in:   "",
			want: `FROM`,
		},
		{
			in:   "centos:latest",
			want: `FROM centos:latest`,
		},
		{
			in:   "中关村",
			want: `FROM 中关村`,
		},
		{
			in:   "centos\nRUN rm -rf /\n\nUSER 100",
			want: `FROM centos RUN rm -rf /  USER 100`,
		},
	}
	for _, tc := range testCases {
		got, err := From(tc.in)
		if err != nil {
			t.Fatal(err)
		}
		if got != tc.want {
			t.Errorf("From(%v) = %q; want %q", tc.in, got, tc.want)
		}
	}
}
