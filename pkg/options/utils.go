package options

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/gosuri/uitable"
	"github.com/kungze/quic-tun/pkg/log"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func PrintWorkingDir() {
	wd, _ := os.Getwd()
	log.Infof("WorkingDir: %s", wd)
}

// PrintFlags logs the flags in the flagset.
func PrintFlags(flags *pflag.FlagSet) {
	flags.VisitAll(func(flag *pflag.Flag) {
		log.Infof("FLAG: --%s=%q", flag.Name, flag.Value)
	})
}

func PrintConfig() {
	if keys := viper.AllKeys(); len(keys) > 0 {
		fmt.Printf("%v Configuration items:\n", "==>")
		table := uitable.New()
		table.Separator = " "
		table.MaxColWidth = 80
		table.RightAlign(0)
		for _, k := range keys {
			table.AddRow(fmt.Sprintf("%s:", k), viper.Get(k))
		}
		fmt.Printf("%v", table)
		fmt.Println()
	}
}

// HomeDir returns the home directory for the current user.
// On Windows:
// 1. the first of %HOME%, %HOMEDRIVE%%HOMEPATH%, %USERPROFILE% containing a `.apimachinery\config` file is returned.
// 2. if none of those locations contain a `.apimachinery\config` file, the first of
// %HOME%, %USERPROFILE%, %HOMEDRIVE%%HOMEPATH% that exists and is writeable is returned.
// 3. if none of those locations are writeable, the first of %HOME%, %USERPROFILE%,
// %HOMEDRIVE%%HOMEPATH% that exists is returned.
// 4. if none of those locations exists, the first of %HOME%, %USERPROFILE%,
// %HOMEDRIVE%%HOMEPATH% that is set is returned.
func HomeDir() string {
	if runtime.GOOS != "windows" {
		return os.Getenv("HOME")
	}
	home := os.Getenv("HOME")
	homeDriveHomePath := ""
	if homeDrive, homePath := os.Getenv("HOMEDRIVE"), os.Getenv("HOMEPATH"); len(homeDrive) > 0 && len(homePath) > 0 {
		homeDriveHomePath = homeDrive + homePath
	}
	userProfile := os.Getenv("USERPROFILE")

	// Return first of %HOME%, %HOMEDRIVE%/%HOMEPATH%, %USERPROFILE% that contains a `.apimachinery\config` file.
	// %HOMEDRIVE%/%HOMEPATH% is preferred over %USERPROFILE% for backwards-compatibility.
	for _, p := range []string{home, homeDriveHomePath, userProfile} {
		if len(p) == 0 {
			continue
		}
		if _, err := os.Stat(filepath.Join(p, ".apimachinery", "config")); err != nil {
			continue
		}
		return p
	}

	firstSetPath := ""
	firstExistingPath := ""

	// Prefer %USERPROFILE% over %HOMEDRIVE%/%HOMEPATH% for compatibility with other auth-writing tools
	for _, p := range []string{home, userProfile, homeDriveHomePath} {
		if len(p) == 0 {
			continue
		}
		if len(firstSetPath) == 0 {
			// remember the first path that is set
			firstSetPath = p
		}
		info, err := os.Stat(p)
		if err != nil {
			continue
		}
		if len(firstExistingPath) == 0 {
			// remember the first path that exists
			firstExistingPath = p
		}
		if info.IsDir() && info.Mode().Perm()&(1<<(uint(7))) != 0 {
			// return first path that is writeable
			return p
		}
	}

	// If none are writeable, return first location that exists
	if len(firstExistingPath) > 0 {
		return firstExistingPath
	}

	// If none exist, return first location that is set
	if len(firstSetPath) > 0 {
		return firstSetPath
	}

	// We've got nothing
	return ""
}
