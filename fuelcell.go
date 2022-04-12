// Package fuelcell is responsible for integrating command line interactions into
// an easy to workflow that can me adopted into a wide variety of applications
package fuelcell

// ShellCompDirective is a bit map representing the different behaviors the shell
// can be instructed to have once completions have been provided.
type ShellCompDirective int

type PositionalArgs func(cmd *Cmd, args []string) error
