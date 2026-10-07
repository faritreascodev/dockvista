//go:build !windows

package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "dockvista-tray is the Windows helper; on this OS run dockvista directly")
	os.Exit(1)
}
