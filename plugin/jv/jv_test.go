package plugin_jv

import (
	"io"
	"os"
	"strings"
	"testing"
)

func TestRunAcceptsPositionalInput(t *testing.T) {
	j := &Jv{}
	if err := j.Init(); err != nil {
		t.Fatal(err)
	}

	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer read.Close()
	original := os.Stdout
	os.Stdout = write
	t.Cleanup(func() { os.Stdout = original })

	runErr := j.Run([]string{"-f", "-raw", `{"name":"jv"}`})
	os.Stdout = original
	if err := write.Close(); err != nil {
		t.Fatal(err)
	}
	out, err := io.ReadAll(read)
	if err != nil {
		t.Fatal(err)
	}
	if runErr != nil {
		t.Fatal(runErr)
	}
	if got := string(out); !strings.Contains(got, `"name": "jv"`) {
		t.Errorf("formatted positional input = %q", got)
	}
}
