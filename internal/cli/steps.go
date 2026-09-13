package cli

import (
	"fmt"
	"io"
)

func step(w io.Writer, format string, args ...any) {
	fmt.Fprintf(w, "==> "+format+"\n", args...)
}

func fail(w io.Writer, err error) int {
	fmt.Fprintln(w, err)
	return 1
}
