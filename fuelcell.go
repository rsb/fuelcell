// Package fuelcell is responsible for integrating command line interactions into
// an easy to workflow that can me adopted into a wide variety of applications
package fuelcell

type PositionalArgs func(cmd *Cmd, args []string) error
