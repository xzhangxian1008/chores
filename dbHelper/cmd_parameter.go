package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
)

const (
	insertTaskName         = "insert"
	compareResultsTaskName = "compare-results"
	runSQLsTaskName        = "run-sqls"
)

type optionalCLIString struct {
	value string
	set   bool
}

type dbCLIOverrides struct {
	address optionalCLIString
	port    optionalCLIString
	user    optionalCLIString
	dbName  optionalCLIString
}

func (overrides dbCLIOverrides) any() bool {
	return overrides.address.set || overrides.port.set || overrides.user.set || overrides.dbName.set
}

type commandLineOptions struct {
	taskName    string
	dbOverrides dbCLIOverrides
}

func parseTaskName(args []string) (string, error) {
	options, err := parseCommandLineOptions(args)
	if err != nil {
		return "", err
	}
	return options.taskName, nil
}

func parseCommandLineOptions(args []string) (commandLineOptions, error) {
	flags := flag.NewFlagSet("db_helper", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	taskFlag := flags.String("task", "", "task to run: insert, compare-results, or run-sqls")
	addressFlag := flags.String("address", "", "database address")
	portFlag := flags.String("port", "", "database port")
	userFlag := flags.String("user", "", "database user")
	dbNameFlag := flags.String("dbName", "", "database name")
	parseArgs := args

	// Also accept the convenient positional form:
	// db_helper compare-results --address 127.0.0.1 ...
	// The standard flag package stops at the first positional argument, so move
	// a leading task name behind the flags before parsing.
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		parseArgs = append(append([]string(nil), args[1:]...), args[0])
	}
	if err := flags.Parse(parseArgs); err != nil {
		return commandLineOptions{}, fmt.Errorf("parse arguments: %w", err)
	}
	visited := make(map[string]bool)
	flags.Visit(func(current *flag.Flag) {
		visited[current.Name] = true
	})
	taskName := strings.TrimSpace(*taskFlag)
	positional := flags.Args()
	if taskName != "" && len(positional) > 0 {
		return commandLineOptions{}, errors.New("specify the task either with --task or as a positional argument, not both")
	}
	if taskName == "" {
		if len(positional) == 0 {
			return commandLineOptions{}, errors.New("please choose a task: --task insert, --task compare-results, or --task run-sqls")
		}
		if len(positional) > 1 {
			return commandLineOptions{}, errors.New("only one task may be specified")
		}
		taskName = positional[0]
	}

	taskName = strings.ToLower(strings.ReplaceAll(strings.TrimSpace(taskName), "_", "-"))
	switch taskName {
	case insertTaskName:
		taskName = insertTaskName
	case "compare", "compare-result", compareResultsTaskName:
		taskName = compareResultsTaskName
	case "run-sql", runSQLsTaskName:
		taskName = runSQLsTaskName
	default:
		return commandLineOptions{}, fmt.Errorf("unsupported task %q; choose insert, compare-results, or run-sqls", taskName)
	}

	return commandLineOptions{
		taskName: taskName,
		dbOverrides: dbCLIOverrides{
			address: optionalCLIString{value: *addressFlag, set: visited["address"]},
			port:    optionalCLIString{value: *portFlag, set: visited["port"]},
			user:    optionalCLIString{value: *userFlag, set: visited["user"]},
			dbName:  optionalCLIString{value: *dbNameFlag, set: visited["dbName"]},
		},
	}, nil
}
