package main

import (
	"flag"
	"fmt"
	"os"
)

type appFlags struct {
	scanTargetsFile    string
	monitorTargetsFile string
	globalConfigFile   string
	mode               string
}

func parseFlags() appFlags {
	scanTargetsFile := flag.String("scan-targets", "", "Path to a text file containing seed URLs for the main scan. Used if --diff-target-file is not set. This flag is for backward compatibility.")
	scanTargetsFileAlias := flag.String("st", "", "Alias for -scan-targets")

	monitorTargetsFile := flag.String("monitor-targets", "", "Path to a text file containing JS/HTML URLs for file monitoring (only in automated mode).")
	monitorTargetsFileAlias := flag.String("mt", "", "Alias for --monitor-targets")

	globalConfigFile := flag.String("globalconfig", "", "Path to the global YAML/JSON configuration file. If not set, searches default locations.")
	globalConfigFileAlias := flag.String("gc", "", "Alias for --globalconfig")

	modeFlag := flag.String("mode", "", "Mode to run the tool: onetime or automated (overrides config file if set)")
	modeFlagAlias := flag.String("m", "", "Alias for --mode")

	flag.Parse()

	flags := appFlags{}

	if *scanTargetsFile != "" {
		flags.scanTargetsFile = *scanTargetsFile
	} else if *scanTargetsFileAlias != "" {
		flags.scanTargetsFile = *scanTargetsFileAlias
	}

	if *monitorTargetsFile != "" {
		flags.monitorTargetsFile = *monitorTargetsFile
	} else if *monitorTargetsFileAlias != "" {
		flags.monitorTargetsFile = *monitorTargetsFileAlias
	}

	if *globalConfigFile != "" {
		flags.globalConfigFile = *globalConfigFile
	} else if *globalConfigFileAlias != "" {
		flags.globalConfigFile = *globalConfigFileAlias
	}

	if *modeFlag != "" {
		flags.mode = *modeFlag
	} else if *modeFlagAlias != "" {
		flags.mode = *modeFlagAlias
	}

	// Auto-set mode to automated if using monitor-specific flags
	if flags.mode == "" {
		if flags.monitorTargetsFile != "" {
			flags.mode = "automated"
			fmt.Printf("[INFO] Mode automatically set to 'automated' due to monitor-related flags\n")
		} else {
			fmt.Fprintln(os.Stderr, "[FATAL] --mode argument is required (onetime or automated)")
			os.Exit(1)
		}
	}

	// Validate flag combinations
	if err := validateFlags(flags); err != nil {
		fmt.Fprintf(os.Stderr, "[FATAL] %v\n", err)
		os.Exit(1)
	}

	return flags
}

// validateFlags validates command line flag combinations
func validateFlags(flags appFlags) error {
	if flags.monitorTargetsFile != "" && flags.mode == "onetime" {
		return fmt.Errorf("-mt (monitor targets) cannot be used with mode 'onetime'. Use 'automated' mode or omit mode flag")
	}

	return nil
}
