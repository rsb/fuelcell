package fuelcell

import (
	"bytes"
	"context"
	"fmt"
	"github.com/rsb/failure"
	flag "github.com/spf13/pflag"
	"io"
	"os"
	"sort"
	"strings"
)

// FParseErrWhitelist configures Flag parse errors to be ignored
type FParseErrWhitelist flag.ParseErrorsWhitelist

// ControlUsageFn is the function signature for the usage closure.
type ControlUsageFn func(*Cmd) error

// ControlFlagErrorFn is a function signature to allow user to control when
// the parsing of a flag returns an error
type ControlFlagErrorFn func(*Cmd, error) error

// ControlHelpFn is a function signature to allow users to control help
type ControlHelpFn func(*Cmd, []string)

// CLIRun defines how a Cmd should be executed when error handle is governed
// by the returned error.
type CLIRun func(*Cmd, []string) error

// GlobalNormalizeFlagFn defined the signature for the global normalization
// function that can be used on every pflag set and children commands
type GlobalNormalizeFlagFn func(f *flag.FlagSet, name string) flag.NormalizedName

// Cmd represents a command on the command line. This command is heavily
// influenced by Cobra cli. The goal of this project is to implement what
// cobra did but with a few difference and of course remove unneeded legacy
// baggage.
type Cmd struct {
	// Use is the one-line usage message.
	// Recommended syntax is as follows:
	//   [ ] identifies an optional argument. Arguments that are not enclosed in brackets are required.
	//   ... indicates that you can specify multiple values for the previous argument.
	//   |   indicates mutually exclusive information. You can use the argument to the left of the separator or the
	//       argument to the right of the separator. You cannot use both arguments in a single use of the command.
	//   { } delimits a set of mutually exclusive arguments when one of the arguments is required. If the arguments are
	//       optional, they are enclosed in brackets ([ ]).
	// Example: add [-F file | -D dir]... [-f format] profile
	Use string

	// Aliases is an array of aliases that can be used instead of the first word in Use.
	Aliases []string

	// SuggestFor is an array of command names for which this command will be suggested -
	// similar to aliases but only suggests.
	SuggestFor []string

	// Short is the short description shown in the 'help' output.
	Short string

	// Long is the long message shown in the 'help <this-command>' output.
	Long string

	// Example is examples of how to use the command.
	Example string

	// ValidArgs is list of all valid non-flag arguments that are accepted in shell completions
	ValidArgs []string

	// ValidArgsFunction is an optional function that provides valid non-flag arguments for shell completion.
	// It is a dynamic version of using ValidArgs.
	// Only one of ValidArgs and ValidArgsFunction can be used for a command.
	ValidArgsFunction func(cmd *Cmd, args []string, toComplete string) ([]string, ShellCompDirective)

	// Expected arguments
	Args PositionalArgs

	// ArgAliases is List of aliases for ValidArgs.
	// These are not suggested to the user in the shell completion,
	// but accepted if entered manually.
	ArgAliases []string

	// Deprecated defines, if this command is deprecated and should print this string when used.
	Deprecated string

	// Version defines the version for this command. If this value is non-empty and the command does not
	// define a "version" flag, a "version" boolean flag will be added to the command and, if specified,
	// will print content of the "Version" variable. A shorthand "v" flag will also be added if the
	// command does not define one.
	Version string

	// The *Run functions are executed in the following order:
	//   * PersistentPreRun()
	//   * PreRun()
	//   * Run()
	//   * PostRun()
	//   * PersistentPostRun()
	// All functions get the same args, the arguments after the command name.
	//

	// The run event function are executed in the following order:
	// * GlobalPreRun
	// * PreRun
	// * Run
	// * PostRun
	// * GlobalPostRun
	// All function have the same run signature CLIRun
	lifecycle Lifecycle

	// args is actual args parsed from flags.
	args []string

	// Manage all the pflags
	flags Flags

	// Controls the usage string
	usage Usage

	// flagErrorFn is func defined by user, and it's called when the parsing of
	// flags returns an error.
	flagErrorFn ControlFlagErrorFn

	// help allows for the configuration of the help message by the user
	help Help

	// versionTemplate is the version template defined by user.
	versionTemplate string

	// input, output and error streams
	streams DataStreams

	// FParseErrWhitelist flag parse errors to be ignored
	FParseErrWhitelist FParseErrWhitelist

	// CompletionOptions is a set of options to control the handling of shell completion
	CompletionOptions CompletionOptions

	// isSortedCmds defines, if command slice are sorted or not.
	isSortedCmds bool

	// calledAs is the name or alias value used to call this command.
	calledAs CalledAs

	ctx context.Context

	// commands is the list of commands supported by this program.
	commands []*Cmd
	// parent is a parent command for this command.
	parent *Cmd

	// Max lengths of commands' string lengths for use in padding.
	maxLength MaxLengths

	// TraverseChildren parses flags on all parents before executing child command.
	TraverseChildren bool

	// Hidden defines, if this command is hidden and should NOT show up in the list of available commands.
	Hidden bool

	// SilenceErrors is an option to quiet errors down stream.
	SilenceErrors bool

	// SilenceUsage is an option to silence usage when an error occurs.
	SilenceUsage bool

	// DisableFlagParsing disables the flag parsing.
	// If this is true all flags will be passed to the command as arguments.
	DisableFlagParsing bool

	// DisableAutoGenTag defines, if gen tag ("Auto generated by spf13/cobra...")
	// will be printed by generating docs for this command.
	DisableAutoGenTag bool

	// DisableFlagsInUseLine will disable the addition of [flags] to the usage
	// line of a command when printing help or generating docs
	DisableFlagsInUseLine bool

	// DisableSuggestions disables the suggestions based on Levenshtein distance
	// that go along with 'unknown command' messages.
	DisableSuggestions bool

	// SuggestionsMinimumDistance defines minimum levenshtein distance to display suggestions.
	// Must be > 0.
	SuggestionsMinimumDistance int
}

// Find the target command given the args and the cmd tree.
// This should be run on the highest node. Only searches down.
func (c *Cmd) Find(args []string) (*Cmd, []string, error) {
	var innerFind func(*Cmd, []string) (*Cmd, []string)

	innerFind = func(c *Cmd, innerArgs []string) (*Cmd, []string) {
		argsWOflags := stripFlags(innerArgs, c)
		if len(argsWOflags) == 0 {
			return c, innerArgs
		}
		nextSubCmd := argsWOflags[0]

		cmd := c.findNext(nextSubCmd)
		if cmd != nil {
			return innerFind(cmd, argsMinusFirstX(innerArgs, nextSubCmd))
		}
		return c, innerArgs
	}

	found, a := innerFind(c, args)
	if found.Args == nil {
		return found, a, legacyArgs(found, stripFlags(a, found))
	}

	return found, a, nil
}

func (c *Cmd) preRun() {
	for _, x := range initializers {
		x()
	}
}

func (c *Cmd) Execute() error {
	_, err := c.ExecuteC()
	return err
}

func (c *Cmd) ExecuteC() (*Cmd, error) {
	if c.ctx == nil {
		c.ctx = context.Background()
	}

	// Regardless of what command execute is called on, run on Root only.
	if c.HasParent() {
		return c.Root().ExecuteC()
	}

	// initialize help at the last point to allow for user overriding.
	return nil, nil
}

func (c *Cmd) execute(a []string) (err error) {
	if c == nil {
		return failure.System("can not execute on a Cmd that is nil")
	}

	streams := c.streams

	if len(c.Deprecated) > 0 {
		streams.Printf("Command %q is deprecated, %s\n", c.Deprecated)
	}

	// initialize help and version flag at the last point possible to allow
	// for user overriding.
	c.InitDefaultHelpFlag()
	c.InitDefaultVersionFlag()

	if err = c.ParseFlags(a); err != nil {
		return c.FlagErrorFn()(c, err)
	}

	// If help is called, regardless of the other flags, return we want help.
	// Also say we need help if the command is not runnable.
	helpValue, err := c.Flags().GetBool("help")
	if err != nil {
		// should be impossible to get here as we always declare a help
		// flag in InitDefaultHelpFlag()
		streams.Println("\"help\" flag declared as non-bool. Please correct your code.")
		return failure.ToSystem(err, "c.Flags().GetBool(help) failed")
	}

	if helpValue {
		return flag.ErrHelp
	}

	if c.Version != "" {
		versionValue, err := c.Flags().GetBool("version")
		if err != nil {
			streams.Println("\"help\" flag declared as non-bool. Please correct your code.")
			return failure.ToSystem(err, "c.Flags().GetBool(version) failed")
		}

		if versionValue {
			err := tpl(streams.Out(), c.VersionTemplate(), c)
			if err != nil {
				streams.Println(err)
			}
			return err
		}
	}

	events := c.lifecycle

	if !events.IsRunnable() {
		return flag.ErrHelp
	}

	c.preRun()

	argWoFlags := c.Flags().Args()
	if c.DisableFlagParsing {
		argWoFlags = a
	}

	if err := c.ValidateArgs(argWoFlags); err != nil {
		return err
	}

	for p := c; p != nil; p = p.Parent() {
		if p.lifecycle.GlobalPreRun != nil {
			if err := p.lifecycle.GlobalPreRun(c, argWoFlags); err != nil {
				return err
			}
			break
		}
	}

	if c.lifecycle.PreRun != nil {
		if err := c.lifecycle.PreRun(c, argWoFlags); err != nil {
			return err
		}
	}

	if err := c.validateRequiredFlags(); err != nil {
		return err
	}

	if c.lifecycle.Run != nil {
		if err := c.lifecycle.Run(c, argWoFlags); err != nil {
			return err
		}
	}

	if c.lifecycle.PostRun != nil {
		if err := c.lifecycle.PostRun(c, argWoFlags); err != nil {
			return err
		}
	}

	for p := c; p != nil; p = p.Parent() {
		if p.lifecycle.GlobalPostRun != nil {
			if err := p.lifecycle.GlobalPostRun(c, argWoFlags); err != nil {
				return err
			}
			break
		}
	}

	return nil
}

// InitDefaultHelpCmd adds default help command to this command.
// It is called automatically by executing the cmd or by calling help
// and usage. Ignored if cmd already has help or no subcommands
func (c *Cmd) InitDefaultHelpCmd() {
	if !c.HasSubCommands() {
		return
	}

	if c.help.Default == nil {

	}
}

func (c *Cmd) ValidateArgs(args []string) error {
	if c.Args == nil {
		return nil
	}

	return c.Args(c, args)

}

// VersionTemplate return version template for the command.
func (c *Cmd) VersionTemplate() string {
	if c.versionTemplate != "" {
		return c.versionTemplate
	}

	if c.HasParent() {
		return c.parent.VersionTemplate()
	}

	return `{{with .Name}}{{printf "%s " .}}{{end}}{{printf "version %s" .Version}}
`
}

// InitDefaultHelpFlag adds default help flag to this command.
// It is called automatically by executing this command or by
// calling help and usage. When help exists nothing is done.
func (c *Cmd) InitDefaultHelpFlag() {
	c.mergeGlobalFlags()
	if c.Flags().Lookup("help") == nil {
		usage := "help for "
		if c.Name() == "" {
			usage += "this command"
		} else {
			usage += c.Name()
		}
		c.Flags().BoolP("help", "h", false, usage)
	}
}

func (c *Cmd) InitDefaultVersionFlag() {
	if c.Version == "" {
		return
	}

	c.mergeGlobalFlags()
	if c.Flags().Lookup("version") == nil {
		usage := "version for "
		if c.Name() == "" {
			usage += "this command"
		} else {
			usage += c.Name()
		}

		if c.Flags().ShorthandLookup("v") == nil {
			c.Flags().BoolP("version", "v", false, usage)
		} else {
			c.Flags().Bool("version", false, usage)
		}
	}
}

// Context returns underlying command context. If command was executed
// with ExecuteContext or the context was set with SetContext, the
// previously set context will be returned. Otherwise, nil is returned.
//
// Notice that a call to Execute and ExecuteC will replace a nil context of
// a command with a context.Background, so a background context will be
// returned by Context after one of these functions has been called.
func (c *Cmd) Context() context.Context {
	return c.ctx
}

// SetContext sets context for the command. It is set to context.Background
// by default and will be overwritten by Command.ExecuteContext or
// Command.ExecuteContextC
func (c *Cmd) SetContext(ctx context.Context) {
	c.ctx = ctx
}

// SetArgs sets arguments for the command. It is set to os.Args[1:] by default,
// if desired, can be overridden particularly useful when testing.
func (c *Cmd) SetArgs(a []string) {
	c.args = a
}

// InputStream returns the assign stdin
func (c *Cmd) InputStream() io.Reader {
	return c.streams.In()
}

// SetInputStream allows the input stream to be assigned to the command.
func (c *Cmd) SetInputStream(in io.Reader) {
	c.streams.SetIn(in)
}

// OutputStream returns the assign stdout
func (c *Cmd) OutputStream() io.Writer {
	return c.streams.Out()
}

// SetOutputStream allows the output stream to be assigned to the command.
func (c *Cmd) SetOutputStream(out io.Writer) {
	c.streams.SetOut(out)
}

// ErrorStream returns the assign stderr
func (c *Cmd) ErrorStream() io.Writer {
	return c.streams.Error()
}

// SetErrorStream allows the error stream to be assigned to the command.
func (c *Cmd) SetErrorStream(e io.Writer) {
	c.streams.SetError(e)
}

// SetUsageClosure assign user defined closure for usage
func (c *Cmd) SetUsageClosure(fn ControlUsageFn) {
	c.usage.Control = fn
}

// SetUsageTemplate allows the user to control the usage template.
func (c *Cmd) SetUsageTemplate(s string) {
	c.usage.Template = s
}

// Parent returns this commands parent command.
func (c *Cmd) Parent() *Cmd {
	return c.parent
}

// NameAndAliases returns a list of the command name and all aliases
func (c *Cmd) NameAndAliases() string {
	return strings.Join(append([]string{c.Name()}, c.Aliases...), ",")
}

// HasExample determines if the command has examples
func (c *Cmd) HasExample() bool {
	return len(c.Example) > 0
}

// HasParent determines if the command is a child
func (c *Cmd) HasParent() bool {
	return c.parent != nil
}

// HasSubCommands determines if the command has any child commands.
func (c *Cmd) HasSubCommands() bool {
	return len(c.commands) > 0
}

// Commands returns a sorted slice of child commands
func (c *Cmd) Commands() []*Cmd {
	if !c.isCommandsSorted() {
		sort.Sort(sortByName(c.commands))
		c.isSortedCmds = true
	}
	return c.commands
}

// ResetCommands delete parent, subcommands, and help command from this cmd.
func (c *Cmd) ResetCommands() {
	c.parent = nil
	c.commands = nil
	c.help.ClearDefault()
	c.flags.ClearParentsGlobal()
}

// Name returns the command's name: the first word in the use line
func (c *Cmd) Name() string {
	name := c.Use
	i := strings.Index(name, " ")
	if i >= 0 {
		name = name[:i]
	}

	return name
}

// HasAlias determines if a given string is an alias of a command.
func (c *Cmd) HasAlias(s string) bool {
	for _, a := range c.Aliases {
		if a == s {
			return true
		}
	}
	return false
}

// Path return the full path to this command.
func (c *Cmd) Path() string {
	if c.HasParent() {
		return c.Parent().Path() + " " + c.Name()
	}

	return c.Name()
}

// Root finds the root command.
func (c *Cmd) Root() *Cmd {
	if c.HasParent() {
		return c.Parent().Root()
	}

	return c
}

// VisitParents visits all parents of the command and invokes fn on each
func (c *Cmd) VisitParents(fn func(*Cmd)) {
	if !c.HasParent() {
		return
	}

	fn(c.Parent())
	c.Parent().VisitParents(fn)
}

// Flags returns the complete FlagSet that applies to this command
// (local and global declared here by all parents)
func (c *Cmd) Flags() *flag.FlagSet {
	if !c.flags.IsFull() {
		c.flags.LoadFullSet(c.Name())
	}

	return c.flags.Full
}

// GlobalFlags returns the persistent FlagSet specifically set in the
// current command
func (c *Cmd) GlobalFlags() *flag.FlagSet {
	if !c.flags.IsGlobal() {
		c.flags.LoadGlobalSet(c.Name())
	}

	return c.flags.Global
}

// LocalSpecificFlags are flags specific to this command which will NOT
// persist to subcommands.
func (c *Cmd) LocalSpecificFlags() *flag.FlagSet {
	return nil
}

// LocalFlags returns the local FlagSet specifically set in the current command.
func (c *Cmd) LocalFlags() *flag.FlagSet {
	return nil
}

func (c *Cmd) FlagErrorFn() ControlFlagErrorFn {
	if c.flagErrorFn != nil {
		return c.flagErrorFn
	}

	if c.HasParent() {
		return c.parent.FlagErrorFn()
	}

	return func(c *Cmd, err error) error { return err }
}

// ParseFlags parses global and local flags
func (c *Cmd) ParseFlags(args []string) error {
	if c.DisableFlagParsing {
		return nil
	}

	errorBuf := c.flags.LoadErrorBufferWhenEmpty()
	beforeErrorLen := errorBuf.Len()

	c.mergeGlobalFlags()

	// do it here after merging all the flags and just before parse
	c.Flags().ParseErrorsWhitelist = flag.ParseErrorsWhitelist(c.FParseErrWhitelist)

	err := c.Flags().Parse(args)
	// Print warnings if they occurred (e.g. deprecated flag messages).
	if errorBuf.Len()-beforeErrorLen > 0 && err == nil {
		// c.Print(errorBuf.String())
	}
	return err
}

// mergeGlobalFlags merges c.flags.Global into c.flags.Full
// and adds missing global flags to all parents.
func (c *Cmd) mergeGlobalFlags() {
	c.updateParentGlobalFlags()
	c.Flags().AddFlagSet(c.GlobalFlags())
	c.Flags().AddFlagSet(c.flags.ParentsGlobal)
}

// updateParentGlobalFlags updates flags.ParentsGlobal by
// adding global flags for all parents
// If c.flags.ParentsGlobal is nil it makes new.
func (c *Cmd) updateParentGlobalFlags() {
	if !c.flags.IsParentsGlobalFlags() {
		c.flags.LoadParentsGlobal(c.Name())
	}

	if c.flags.IsGlobalNormalizeFn() {
		c.flags.ParentsGlobal.SetNormalizeFunc(c.flags.GlobalNormalizeFn)
	}

	c.Root().GlobalFlags().AddFlagSet(flag.CommandLine)

	c.VisitParents(func(parent *Cmd) {
		c.flags.ParentsGlobal.AddFlagSet(parent.GlobalFlags())
	})
}

func (c *Cmd) HasAvailableFlags() bool {
	return c.Flags().HasAvailableFlags()
}

func (c *Cmd) UseLine() string {
	line := c.Use
	if c.HasParent() {
		line = c.parent.Path() + " " + c.Use
	}

	if c.DisableFlagsInUseLine {
		return line
	}

	if c.HasAvailableFlags() && !strings.Contains(line, "[flags]") {
		line += " [flags]"
	}

	return line
}

// IsGlobalNormalizationEnabled determines if the closure is set
func (c *Cmd) IsGlobalNormalizationEnabled() bool {
	return c.flags.GlobalNormalizeFn != nil
}

// GlobalNormalization return the GlobalNormalizeFlagFn closure
func (c *Cmd) GlobalNormalization() GlobalNormalizeFlagFn {
	return c.flags.GlobalNormalizeFn
}

// SetGlobalNormalization assigns the closure to the command
func (c *Cmd) SetGlobalNormalization(fn GlobalNormalizeFlagFn) {
	c.flags.GlobalNormalizeFn = fn
}

func (c *Cmd) markCommandsSorted() {
	c.isSortedCmds = true
}

func (c *Cmd) markCommandsUnsorted() {
	c.isSortedCmds = false
}

func (c *Cmd) isCommandsSorted() bool {
	return c.isSortedCmds
}

func (c *Cmd) resetMaxLengths() {
	c.maxLength.Reset()
}

func (c *Cmd) updateMaxLengthFrom(child *Cmd) {
	usageLen := len(child.Use)
	if usageLen > c.maxLength.Use {
		c.maxLength.Use = usageLen
	}

	pathLen := len(child.Path())
	if pathLen > c.maxLength.Path {
		c.maxLength.Path = pathLen
	}

	nameLen := len(child.Name())
	if nameLen > c.maxLength.Name {
		c.maxLength.Name = nameLen
	}
}
func (c *Cmd) findNext(next string) *Cmd {
	matches := make([]*Cmd, 0)
	for _, cmd := range c.commands {
		if cmd.Name() == next || cmd.HasAlias(next) {
			cmd.calledAs.Name = next
			return cmd
		}
	}

	if len(matches) == 1 {
		return matches[0]
	}

	return nil
}

func (c *Cmd) validateRequiredFlags() error {
	if c.DisableFlagParsing {
		return nil
	}

	flags := c.Flags()
	var missing []string
	flags.VisitAll(func(pflag *flag.Flag) {
		requiredAnnotation, found := pflag.Annotations[BashCompOneRequiredFlag]
		if !found {
			return
		}
		if (requiredAnnotation[0] == "true") && !pflag.Changed {
			missing = append(missing, pflag.Name)
		}
	})

	if len(missing) > 0 {
		return failure.System(`required flag(s) "%s" not set`, strings.Join(missing, `","`))
	}

	return nil
}

// Add assigns on or more commands to this parent command
// NOTE: this will panic if you try to add a command to itself
func (c *Cmd) Add(cmds ...*Cmd) {
	for i, x := range cmds {
		if cmds[i] == c {
			panic("[Add Failed] Command can't be a child of itself")
		}

		cmds[i].parent = c
		c.updateMaxLengthFrom(x)
		if c.IsGlobalNormalizationEnabled() {
			x.SetGlobalNormalization(c.GlobalNormalization())
		}

		c.commands = append(c.commands, x)
		c.markCommandsUnsorted()
	}
}

// Remove removes one or more commands from the parent command.
func (c *Cmd) Remove(cmds ...*Cmd) {
	var commands []*Cmd

MAIN:
	for _, command := range c.commands {
		for _, cmd := range cmds {
			if command == cmd {
				command.parent = nil
				continue MAIN
			}
		}
		commands = append(commands, command)
	}

	// recompute all lengths
	c.resetMaxLengths()
	for _, command := range c.commands {
		c.updateMaxLengthFrom(command)
	}
}

// DataStreams represents the 3 modes by which data travels via the cli.
// In: 	the standard input os.Stdin of the app.
// Out:	the standard output os.Stdout of the app.
// Err: the standard error os.Stderr of the app.
//
// These can all be controlled by the user, but left on touched the defaults
// are listed as above
type DataStreams struct {
	in  io.Reader
	out io.Writer
	err io.Writer
}

// NewDataStreams constructor used to create in/out and err streams
func NewDataStreams(in io.Reader, out, err io.Writer) DataStreams {
	return DataStreams{
		in:  in,
		out: out,
		err: err,
	}
}

func (ds *DataStreams) SetIn(in io.Reader) {
	ds.in = in
}

func (ds *DataStreams) In() io.Reader {
	if ds.in == nil {
		ds.in = os.Stdin
	}
	return ds.in
}

func (ds *DataStreams) SetOut(out io.Writer) {
	ds.out = out
}

func (ds *DataStreams) Out() io.Writer {
	if ds.out == nil {
		ds.out = os.Stdout
	}
	return ds.out
}

func (ds *DataStreams) SetError(err io.Writer) {
	ds.err = err
}

func (ds *DataStreams) Error() io.Writer {
	if ds.err == nil {
		ds.err = os.Stderr
	}

	return ds.err
}

// Print is a convenience method to Print to the DataStreams output
func (ds *DataStreams) Print(i ...interface{}) {
	_, _ = fmt.Fprint(ds.Out(), i...)
}

// Println is a convenience method to Println
func (ds *DataStreams) Println(i ...interface{}) {
	ds.Print(fmt.Sprintln(i...))
}

// Printf is a convenience to Printf
func (ds *DataStreams) Printf(format string, i ...interface{}) {
	ds.Print(fmt.Sprintf(format, i...))
}

// PrintErr is a convenience method to Fprint to the defined Err output
func (ds *DataStreams) PrintErr(i ...interface{}) {
	_, _ = fmt.Fprint(ds.Error(), i...)
}

// PrintErrln is a convenience method to Println to the defined Err output
func (ds *DataStreams) PrintErrln(i ...interface{}) {
	ds.PrintErr(fmt.Sprintln(i...))
}

// PrintErrf is a convenience method to Print
func (ds *DataStreams) PrintErrf(i ...interface{}) {
	ds.Print(fmt.Sprintln(i...))
}

// Usage allows the user to control the usage string in the cli
type Usage struct {
	Control  ControlUsageFn
	Template string
}

// Help allow for the configuration of the cli help screen
// Control: 	help function defined by the user
// Template: 	help template defined by the user
// Default: 	default help cmd
type Help struct {
	Control  ControlHelpFn
	Template string
	Default  *Cmd
}

func (h *Help) ClearDefault() {
	h.Default = nil
}

// MaxLengths store the setting which control the max length of
// Use 	- The usage line.
// Path - The command path. The full path to this command
// Name - The command name. Which is the first word in the usage
//
// This is used in padding.
type MaxLengths struct {
	Use  int
	Path int
	Name int
}

// Reset reverts all lengths to their default values
func (ml MaxLengths) Reset() {
	ml.Use = 0
	ml.Path = 0
	ml.Name = 0
}

// Lifecycle holds all the different events which are fired during the
// lifetime of the command.
// Events are run in the following order:
// * GlobalPreRun
// * PreRun
// * Run
// * PostRun
// * GlobalPostRun
// All events follow the same function signature.
type Lifecycle struct {
	GlobalPreRun  CLIRun
	PreRun        CLIRun
	Run           CLIRun
	PostRun       CLIRun
	GlobalPostRun CLIRun
}

// IsRunnable Determines if a command can be executed.
func (l *Lifecycle) IsRunnable() bool {
	return l.Run != nil
}

// Flags hold all the various flag sets from `github.com/spf13/pflag`
type Flags struct {
	ErrorBuf      *bytes.Buffer
	Full          *flag.FlagSet
	Global        *flag.FlagSet
	Local         *flag.FlagSet
	Inherited     *flag.FlagSet
	ParentsGlobal *flag.FlagSet

	GlobalNormalizeFn GlobalNormalizeFlagFn
}

func (f *Flags) ClearParentsGlobal() {
	f.ParentsGlobal = nil
}

func (f *Flags) IsParentsGlobalFlags() bool {
	return f.ParentsGlobal == nil
}

func (f *Flags) LoadParentsGlobal(name string) {
	f.ParentsGlobal = newFlagSet(name)
	f.ParentsGlobal.SetOutput(f.LoadErrorBufferWhenEmpty())
	f.ParentsGlobal.SortFlags = false
}

func (f *Flags) IsGlobalNormalizeFn() bool {
	return f.GlobalNormalizeFn != nil
}

func (f *Flags) IsErrorBuffer() bool {
	return f.ErrorBuf != nil
}

func (f *Flags) LoadErrorBufferWhenEmpty() *bytes.Buffer {
	if f.IsErrorBuffer() {
		return f.ErrorBuf
	}

	f.LoadErrorBuffer()
	return f.ErrorBuf
}

func (f *Flags) LoadErrorBuffer() {
	f.ErrorBuf = new(bytes.Buffer)
}

func (f *Flags) IsFull() bool {
	return f.Global != nil
}

func (f *Flags) LoadFullSet(name string) {
	f.Full = newFlagSet(name)
	f.Full.SetOutput(f.LoadErrorBufferWhenEmpty())
}

func (f *Flags) IsGlobal() bool {
	return f.Global != nil
}

func (f *Flags) LoadGlobalSet(name string) {
	f.Global = newFlagSet(name)
	f.Global.SetOutput(f.LoadErrorBufferWhenEmpty())
}

// CalledAs is the name of alias used to call a command
type CalledAs struct {
	Name     string
	IsCalled bool
}

func newFlagSet(name string) *flag.FlagSet {
	return flag.NewFlagSet(name, flag.ContinueOnError)
}

type sortByName []*Cmd

func (s sortByName) Len() int           { return len(s) }
func (s sortByName) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s sortByName) Less(i, j int) bool { return s[i].Name() < s[j].Name() }

func NewDefaultHelpCmd(c *Cmd) *Cmd {
	return &Cmd{
		Use:   "help [command]",
		Short: "Help about any command",
		Long: `Help provides help for any command in the application.
Simply type ` + c.Name() + ` help [path to command] for full details`,
		ValidArgs: func(c *Cmd, args []string, toComplete string) ([]string, ShellCompDirective) {
			var completions []string
			cmd, _, e := c.Root().Find(args)
			if e != nil {
				return nil, ShellCompDirectiveNoFileComp
			}

			if cmd == nil {
				// Root help cmd
				cmd = c.Root()
			}

			for _, subCmd := range cmd.Commands() {
			}
		},
	}
}

func stripFlags(args []string, c *Cmd) []string {
	if len(args) == 0 {
		return args
	}

	c.mergeGlobalFlags()

	var commands []string
	flags := c.Flags()

LOOP:
	for len(args) > 0 {
		s := args[0]
		args = args[1:]
		switch {
		case s == "--":
			// "--" terminates the flags
			break LOOP
		case strings.HasPrefix(s, "--") && !strings.Contains(s, "=") && !hasNoOptDefVal(s[2:], flags):
			// If '--flag arg' than
			// delete arg from args.
			fallthrough // (do the same as below)
		case strings.HasPrefix(s, "-") && !strings.Contains(s, "=") && len(s) == 2 && !shortHasNoOptDefVal(s[1:], flags):
			// If '-f arg' then
			// delete 'arg' from args or break the loop if len(args) <= 1
			if len(args) <= 1 {
				break LOOP
			} else {
				args = args[1:]
				continue
			}
		case s != "" && !strings.HasPrefix(s, "-"):
			commands = append(commands, s)
		}
	}

	return commands
}

// argsMinusFirstX removes only the first x from args.  Otherwise, commands
// that look like openshift admin policy add-role-to-user admin my-user, lose
// the admin argument (arg[4]).
func argsMinusFirstX(args []string, x string) []string {
	for i, y := range args {
		if x == y {
			var ret []string
			ret = append(ret, args[:i]...)
			ret = append(ret, args[i+1:]...)
			return ret
		}
	}
	return args
}

func isFlagArg(arg string) bool {
	return (len(arg) >= 3 && arg[1] == '-') ||
		(len(arg) >= 2 && arg[0] == '-' && arg[1] != '-')
}

func hasNoOptDefVal(name string, fs *flag.FlagSet) bool {
	flag := fs.Lookup(name)
	if flag == nil {
		return false
	}

	return flag.NoOptDefVal != ""
}

func shortHasNoOptDefVal(name string, fs *flag.FlagSet) bool {
	if len(name) == 0 {
		return false
	}

	flag := fs.ShorthandLookup(name[:1])
	if flag == nil {
		return false
	}

	return flag.NoOptDefVal != ""
}
