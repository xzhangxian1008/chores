package main

import (
	"fmt"
	"os"
)

func run(args []string) error {
	options, err := parseCommandLineOptions(args)
	if err != nil {
		return err
	}
	switch options.taskName {
	case insertTaskName:
		applyDBCLIOverrides(&insertDBConfig, options.dbOverrides)
	case compareResultsTaskName:
		applyDBCLIOverrides(&compareResultDBConfig, options.dbOverrides)
	case runSQLsTaskName:
		applyDBCLIOverrides(&runSQLDBConfig, options.dbOverrides)
	}

	switch options.taskName {
	case insertTaskName:
		return runInsertValues()
	case compareResultsTaskName:
		return runCompareResults()
	case runSQLsTaskName:
		return runConfiguredSQLs()
	default:
		return fmt.Errorf("unsupported task %q", options.taskName)
	}
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
