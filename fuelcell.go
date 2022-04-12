// Package fuelcell is responsible for integrating command line interactions into
// an easy to workflow that can me adopted into a wide variety of applications
package fuelcell

import (
	"fmt"
	"io"
	"os"
)

type PositionalArgs func(cmd *Cmd, args []string) error

// CheckErr prints the msg with the prefix [Error]: and exists with a
// default code of 1 unless a int is given as the 2nd param
func CheckErr(msg interface{}, exit ...int) {
	if msg == nil {
		return
	}

	_, _ = fmt.Fprintln(os.Stderr, "[Error]:", msg)

	code := 1
	if len(exit) > 0 {
		code = exit[0]
	}

	os.Exit(code)
}

func CheckWriteString(b io.StringWriter, s string, exit ...int) {
	_, err := b.WriteString(s)
	CheckErr(err, exit...)
}
