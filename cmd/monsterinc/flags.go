package main

import (
	"flag"
	"fmt"
	"os"
)

type AppFlags struct {
	ScanTargetsFile  string
	GlobalConfigFile string
	Mode             string
}

func ParseFlags() AppFlags {
	scanTargetsFile := flag.String("file", "", "Path to a text file containing seed URLs for the main scan. Used if --diff-target-file is not set. This flag is for backward compatibility.")
	scanTargetsFileAlias := flag.String("f", "", "Alias for -file")

	globalConfigFile := flag.String("config", "", "Path to the global YAML/JSON configuration file. If not set, searches default locations.")
	globalConfigFileAlias := flag.String("c", "", "Alias for -config")

	modeFlag := flag.String("mode", "", "Mode to run the tool: onetime or automated (overrides config file if set)")
	modeFlagAlias := flag.String("m", "", "Alias for -mode")

	flag.Parse()

	flags := AppFlags{}

	if *scanTargetsFile != "" {
		flags.ScanTargetsFile = *scanTargetsFile
	} else if *scanTargetsFileAlias != "" {
		flags.ScanTargetsFile = *scanTargetsFileAlias
	}

	if *globalConfigFile != "" {
		flags.GlobalConfigFile = *globalConfigFile
	} else if *globalConfigFileAlias != "" {
		flags.GlobalConfigFile = *globalConfigFileAlias
	}

	if *modeFlag != "" {
		flags.Mode = *modeFlag
	} else if *modeFlagAlias != "" {
		flags.Mode = *modeFlagAlias
	}

	if flags.Mode == "" {
		fmt.Fprintln(os.Stderr, "[FATAL] --mode argument is required (onetime or automated)")
		os.Exit(1)
	}

	return flags
}
