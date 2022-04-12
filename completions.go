package fuelcell

const (
	// ShellCompRequestCmd is the name of the hidden command that is used to request
	// completion results from the program.  It is used by the shell completion scripts.
	ShellCompRequestCmd = "__complete"
	// ShellCompNoDescRequestCmd is the name of the hidden command that is used to request
	// completion results without their description.  It is used by the shell completion scripts.
	ShellCompNoDescRequestCmd = "__completeNoDesc"
)

// ShellCompDirective is a bit map representing the different behaviors the shell
// can be instructed to have once completions have been provided.
type ShellCompDirective int

// CompletionOptions are the options to control shell completion
type CompletionOptions struct {
	// DisableDefaultCmd prevents Cobra from creating a default 'completion' command
	DisableDefaultCmd bool
	// DisableNoDescFlag prevents Cobra from creating the '--no-descriptions' flag
	// for shells that support completion descriptions
	DisableNoDescFlag bool
	// DisableDescriptions turns off all completion descriptions for shells
	// that support them
	DisableDescriptions bool
	// HiddenDefaultCmd makes the default 'completion' command hidden
	HiddenDefaultCmd bool
}
