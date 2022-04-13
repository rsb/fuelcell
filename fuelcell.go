// Package fuelcell is responsible for integrating command line interactions into
// an easy to workflow that can me adopted into a wide variety of applications
package fuelcell

import (
	"fmt"
	"io"
	"os"
	"strings"
	"text/template"
	"unicode"
)

type PositionalArgs func(cmd *Cmd, args []string) error

var initializers []func()

var templateFuncs = template.FuncMap{
	"trim":                   strings.TrimSpace,
	"trimRightSpace":         trimRightSpace,
	"trimTrailingWhitespace": trimRightSpace,
	"rpad":                   rpad,
}

// EnableCommandSorting controls sorting of the slice of commands, which is
// turned on by default. To disable sorting, set it to false.
var EnableCommandSorting = true

// CheckErr prints the msg with the prefix [Error]: and exists with a
// default code of 1 unless int is given as the 2nd param
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

func tpl(w io.Writer, text string, data interface{}) error {
	t := template.New("top")
	t.Funcs(templateFuncs)
	template.Must(t.Parse(text))
	return t.Execute(w, data)
}

func rpad(s string, padding int) string {
	t := fmt.Sprintf("%%-%ds", padding)
	return fmt.Sprintf(t, s)
}

func trimRightSpace(s string) string {
	return strings.TrimRightFunc(s, unicode.IsSpace)
}

func CheckWriteString(b io.StringWriter, s string, exit ...int) {
	_, err := b.WriteString(s)
	CheckErr(err, exit...)
}
