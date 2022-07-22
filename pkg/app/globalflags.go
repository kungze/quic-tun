package app

import (
	"flag"
	"fmt"
	"strings"

	"github.com/spf13/pflag"
)

// AddGlobalFlags explicitly registers flags that libraries (log, verflag, etc.) register
// against the global flagsets from "flag".
// We do this in order to prevent unwanted flags from leaking into the component's flagset.
func AddGlobalFlags(fs *pflag.FlagSet, name string) {
	fs.BoolP("help", "h", false, fmt.Sprintf("help for %s", name))
}

// normalize replaces underscores with hyphens
// we should always use hyphens instead of underscores when registering component flags.
func normalize(s string) string {
	return strings.ReplaceAll(s, "_", "-")
}

// Register adds a flag to local that targets the Value associated with the Flag named globalName in flag.CommandLine.
func Register(local *pflag.FlagSet, globalName string) {
	if f := flag.CommandLine.Lookup(globalName); f != nil {
		pflagFlag := pflag.PFlagFromGoFlag(f)
		pflagFlag.Name = normalize(pflagFlag.Name)
		local.AddFlag(pflagFlag)
	} else {
		panic(fmt.Sprintf("failed to find flag in global flagset (flag): %s", globalName))
	}
}
