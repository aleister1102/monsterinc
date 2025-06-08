package main

import (
	"flag"
	"fmt"
	"os"
)

type AppFlags struct {
	ScanTargetsFile    string
	MonitorTargetsFile string
	GlobalConfigFile   string
	Mode               string
}

func ParseFlags() AppFlags {
	scanTargetsFile := flag.String("scan-targets", "", "Path to a text file containing seed URLs for the main scan. Used if --diff-target-file is not set. This flag is for backward compatibility.")
	scanTargetsFileAlias := flag.String("st", "", "Alias for -scan-targets")

	monitorTargetsFile := flag.String("monitor-targets", "", "Path to a text file containing JS/HTML URLs for file monitoring (only in automated mode).")
	monitorTargetsFileAlias := flag.String("mt", "", "Alias for --monitor-targets")

	globalConfigFile := flag.String("globalconfig", "", "Path to the global YAML/JSON configuration file. If not set, searches default locations.")
	globalConfigFileAlias := flag.String("gc", "", "Alias for --globalconfig")

	modeFlag := flag.String("mode", "", "Mode to run the tool: onetime or automated (overrides config file if set)")
	modeFlagAlias := flag.String("m", "", "Alias for --mode")

	flag.Parse()

	flags := AppFlags{}

	if *scanTargetsFile != "" {
		flags.ScanTargetsFile = *scanTargetsFile
	} else if *scanTargetsFileAlias != "" {
		flags.ScanTargetsFile = *scanTargetsFileAlias
	}

	if *monitorTargetsFile != "" {
		flags.MonitorTargetsFile = *monitorTargetsFile
	} else if *monitorTargetsFileAlias != "" {
		flags.MonitorTargetsFile = *monitorTargetsFileAlias
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

	// Auto-set mode to automated if using monitor-specific flags
	if flags.Mode == "" {
		if flags.MonitorTargetsFile != "" {
			flags.Mode = "automated"
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
func validateFlags(flags AppFlags) error {
	if flags.MonitorTargetsFile != "" && flags.Mode == "onetime" {
		return fmt.Errorf("-mt (monitor targets) cannot be used with mode 'onetime'. Use 'automated' mode or omit mode flag")
	}

	return nil
}
