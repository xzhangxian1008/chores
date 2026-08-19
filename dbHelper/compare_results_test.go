package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"
)

func TestCompareResultRowsOptions(t *testing.T) {
	originalSort := compareResultSortRows
	originalCaseSensitive := compareResultCaseSensitive
	t.Cleanup(func() {
		compareResultSortRows = originalSort
		compareResultCaseSensitive = originalCaseSensitive
	})

	expected := []compareRow{
		{{value: "Alpha"}, {isNull: true}},
		{{value: "beta"}, {value: "2"}},
	}
	actual := []compareRow{
		{{value: "BETA"}, {value: "2"}},
		{{value: "alpha"}, {isNull: true}},
	}

	compareResultSortRows = false
	compareResultCaseSensitive = false
	if err := compareResultRows(expected, actual); err == nil {
		t.Fatal("comparison unexpectedly ignored row order")
	}

	compareResultSortRows = true
	if err := compareResultRows(expected, actual); err != nil {
		t.Fatalf("case-insensitive sorted comparison failed: %v", err)
	}

	compareResultCaseSensitive = true
	if err := compareResultRows(expected, actual); err == nil {
		t.Fatal("case-sensitive comparison unexpectedly accepted different casing")
	}

	compareResultCaseSensitive = false
	nullAsText := []compareRow{
		{{value: "BETA"}, {value: "2"}},
		{{value: "alpha"}, {value: "NULL"}},
	}
	if err := compareResultRows(expected, nullAsText); err == nil {
		t.Fatal("comparison unexpectedly treated SQL NULL and the string NULL as equal")
	}
}

func TestCompareResultRowsReportsLengthAndFirstMismatch(t *testing.T) {
	originalSort := compareResultSortRows
	originalCaseSensitive := compareResultCaseSensitive
	compareResultSortRows = false
	compareResultCaseSensitive = true
	t.Cleanup(func() {
		compareResultSortRows = originalSort
		compareResultCaseSensitive = originalCaseSensitive
	})

	if err := compareResultRows([]compareRow{{{value: "one"}}}, nil); err == nil || !strings.Contains(err.Error(), "length not equal, 1 vs 0") {
		t.Fatalf("unexpected length error: %v", err)
	}

	err := compareResultRows(
		[]compareRow{{{value: "same"}}, {{value: "expected"}}},
		[]compareRow{{{value: "same"}}, {{value: "actual"}}},
	)
	if err == nil || !strings.Contains(err.Error(), "row 1") {
		t.Fatalf("unexpected mismatch error: %v", err)
	}
}

func TestCompareResultRowsUsesUnicodeCaseFolding(t *testing.T) {
	originalCaseSensitive := compareResultCaseSensitive
	compareResultCaseSensitive = false
	t.Cleanup(func() {
		compareResultCaseSensitive = originalCaseSensitive
	})

	// Final sigma and ordinary sigma belong to the same Unicode case-fold cycle.
	expected := []compareRow{{{value: "ΟΣ"}}}
	actual := []compareRow{{{value: "ος"}}}
	if err := compareResultRows(expected, actual); err != nil {
		t.Fatalf("Unicode case-insensitive comparison failed: %v", err)
	}
}

func TestWriteCompareFailureFiles(t *testing.T) {
	pair := compareSQLPair{
		name: "tpch1",
		expected: compareSQL{
			setupSQLs: []string{"set expected_mode=1", "set expected_limit=10"},
			query:     "select expected_value from t",
		},
		actual: compareSQL{
			setupSQLs: []string{"set actual_mode=1"},
			query:     "select actual_value from t",
		},
	}
	expectedRows := []compareRow{
		{{value: "expected-1"}, {isNull: true}},
		{{value: "expected-2"}, {value: "2"}},
	}
	actualRows := []compareRow{{{value: "actual-1"}, {value: "1"}}}
	outputDir := t.TempDir()

	if err := writeCompareFailureFiles(outputDir, pair, expectedRows, actualRows); err != nil {
		t.Fatal(err)
	}

	expectedContent, err := os.ReadFile(filepath.Join(outputDir, "tpch1-expected.txt"))
	if err != nil {
		t.Fatal(err)
	}
	actualContent, err := os.ReadFile(filepath.Join(outputDir, "tpch1-actual.txt"))
	if err != nil {
		t.Fatal(err)
	}

	wantExpected := "expected-1\tNULL\nexpected-2\t2\n\n===== SETUP SQLS =====\n-- setup SQL 1\nset expected_mode=1\n-- setup SQL 2\nset expected_limit=10\n\n===== SQL =====\nselect expected_value from t\n"
	if string(expectedContent) != wantExpected {
		t.Fatalf("unexpected expected failure file:\n%s", expectedContent)
	}
	wantActual := "actual-1\t1\n\n===== SETUP SQLS =====\n-- setup SQL 1\nset actual_mode=1\n\n===== SQL =====\nselect actual_value from t\n"
	if string(actualContent) != wantActual {
		t.Fatalf("unexpected actual failure file:\n%s", actualContent)
	}
}

func TestWriteCompareFailureFilesDoesNotOverwriteExistingFiles(t *testing.T) {
	outputDir := t.TempDir()
	existingFiles := map[string]string{
		"tpch1-expected.txt":    "existing expected zero",
		"tpch1-expected(1).txt": "existing expected one",
		"tpch1-actual.txt":      "existing actual zero",
	}
	for fileName, content := range existingFiles {
		if err := os.WriteFile(filepath.Join(outputDir, fileName), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	pair := compareSQLPair{
		name:     "tpch1",
		expected: compareSQL{query: "select expected"},
		actual:   compareSQL{query: "select actual"},
	}
	if err := writeCompareFailureFiles(
		outputDir,
		pair,
		[]compareRow{{{value: "new expected"}}},
		[]compareRow{{{value: "new actual"}}},
	); err != nil {
		t.Fatal(err)
	}

	for fileName, want := range existingFiles {
		got, err := os.ReadFile(filepath.Join(outputDir, fileName))
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != want {
			t.Fatalf("existing file %q was overwritten: got %q, want %q", fileName, got, want)
		}
	}
	for _, fileName := range []string{"tpch1-expected(2).txt", "tpch1-actual(1).txt"} {
		if _, err := os.Stat(filepath.Join(outputDir, fileName)); err != nil {
			t.Fatalf("new failure file %q was not created: %v", fileName, err)
		}
	}
}

func TestLoadCompareSQLPairsFromYAML(t *testing.T) {
	fileName := filepath.Join(t.TempDir(), "compareCases.yaml")
	content := `
- name: tpch1
  expected:
    setupSQLs:
      - set expected_one=1
      - set expected_two=2
    query: select expected_one
  actual:
    setupSQLs:
      - set actual_one=1
    query: select actual_one
- name: tpch2
  expected:
    setupSQLs: []
    query: select expected_two
  actual:
    setupSQLs: []
    query: select actual_two
`
	if err := os.WriteFile(fileName, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	pairs, err := loadCompareSQLPairsFromYAML(fileName)
	if err != nil {
		t.Fatal(err)
	}
	want := []compareSQLPair{
		{
			name: "tpch1",
			expected: compareSQL{
				setupSQLs: []string{"set expected_one=1", "set expected_two=2"},
				query:     "select expected_one",
			},
			actual: compareSQL{
				setupSQLs: []string{"set actual_one=1"},
				query:     "select actual_one",
			},
		},
		{
			name:     "tpch2",
			expected: compareSQL{query: "select expected_two"},
			actual:   compareSQL{query: "select actual_two"},
		},
	}
	if !reflect.DeepEqual(pairs, want) {
		t.Fatalf("unexpected YAML compare cases:\n got: %#v\nwant: %#v", pairs, want)
	}
}

func TestLoadCompareSQLPairsFromYAMLRejectsUnknownFields(t *testing.T) {
	fileName := filepath.Join(t.TempDir(), "compareCases.yaml")
	content := `
- name: typo-case
  expected:
    setupSqls: []
    query: select 1
  actual:
    setupSQLs: []
    query: select 1
`
	if err := os.WriteFile(fileName, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := loadCompareSQLPairsFromYAML(fileName)
	if err == nil || !strings.Contains(err.Error(), "setupSqls") {
		t.Fatalf("unexpected unknown-field error: %v", err)
	}
}

func TestReadResultFileMatchesPythonReadlinesSemantics(t *testing.T) {
	fileName := filepath.Join(t.TempDir(), "result.txt")
	if err := os.WriteFile(fileName, []byte("first\r\nsecond\rthird"), 0o600); err != nil {
		t.Fatal(err)
	}

	rows, err := readResultFile(fileName)
	if err != nil {
		t.Fatal(err)
	}
	want := []compareRow{
		{{value: "first\n"}},
		{{value: "second\n"}},
		{{value: "third"}},
	}
	if !reflect.DeepEqual(rows, want) {
		t.Fatalf("unexpected rows:\n got: %#v\nwant: %#v", rows, want)
	}
}

func TestCompareConfiguredFilesHonorsSortAndCaseOptions(t *testing.T) {
	tempDir := t.TempDir()
	expectedFile := filepath.Join(tempDir, "expected.txt")
	actualFile := filepath.Join(tempDir, "actual.txt")
	if err := os.WriteFile(expectedFile, []byte("Alpha\nbeta\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(actualFile, []byte("BETA\nalpha\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	originalExpectedFile := compareResultExpectedFile
	originalActualFile := compareResultActualFile
	originalSort := compareResultSortRows
	originalCaseSensitive := compareResultCaseSensitive
	compareResultExpectedFile = expectedFile
	compareResultActualFile = actualFile
	compareResultSortRows = true
	compareResultCaseSensitive = false
	t.Cleanup(func() {
		compareResultExpectedFile = originalExpectedFile
		compareResultActualFile = originalActualFile
		compareResultSortRows = originalSort
		compareResultCaseSensitive = originalCaseSensitive
	})

	if err := compareConfiguredFiles(); err != nil {
		t.Fatalf("configured file comparison failed: %v", err)
	}
}

func TestCompareResultsTaskPrintsAllConfigBeforeRunning(t *testing.T) {
	tempDir := t.TempDir()
	expectedFile := filepath.Join(tempDir, "expected.txt")
	actualFile := filepath.Join(tempDir, "actual.txt")
	for _, fileName := range []string{expectedFile, actualFile} {
		if err := os.WriteFile(fileName, []byte("same\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	originalSort := compareResultSortRows
	originalCaseSensitive := compareResultCaseSensitive
	originalConcurrent := compareResultRunConcurrently
	originalWorkers := compareResultConcurrentWorkerCount
	originalDuration := compareResultConcurrentRunDuration
	originalRunsPerPair := compareResultConcurrentRunsPerPair
	originalReadFromFiles := compareResultReadFromFiles
	originalExpectedFile := compareResultExpectedFile
	originalActualFile := compareResultActualFile
	originalDBConfig := compareResultDBConfig
	originalPairs := compareResultSQLPairs
	t.Cleanup(func() {
		compareResultSortRows = originalSort
		compareResultCaseSensitive = originalCaseSensitive
		compareResultRunConcurrently = originalConcurrent
		compareResultConcurrentWorkerCount = originalWorkers
		compareResultConcurrentRunDuration = originalDuration
		compareResultConcurrentRunsPerPair = originalRunsPerPair
		compareResultReadFromFiles = originalReadFromFiles
		compareResultExpectedFile = originalExpectedFile
		compareResultActualFile = originalActualFile
		compareResultDBConfig = originalDBConfig
		compareResultSQLPairs = originalPairs
	})

	compareResultSortRows = true
	compareResultCaseSensitive = false
	compareResultRunConcurrently = true
	compareResultConcurrentWorkerCount = 7
	compareResultConcurrentRunDuration = 2 * time.Minute
	compareResultConcurrentRunsPerPair = 11
	compareResultReadFromFiles = true
	compareResultExpectedFile = expectedFile
	compareResultActualFile = actualFile
	compareResultDBConfig = dbConfig{
		address: "127.0.0.1",
		port:    "4000",
		user:    "tester",
		dbName:  "compare_db",
		params:  []string{"charset=utf8mb4", "timeout=1s"},
	}
	compareResultSQLPairs = []compareSQLPair{
		{
			name: "config output case",
			expected: compareSQL{
				setupSQLs: []string{"use expected_db", "set x = 1"},
				query:     "select expected_value\nfrom t",
			},
			actual: compareSQL{
				setupSQLs: []string{"use actual_db"},
				query:     "select actual_value from t",
			},
		},
	}

	var compareErr error
	output := captureStdout(t, func() {
		compareErr = compareResultsTask(context.Background())
	})
	if compareErr != nil {
		t.Fatalf("file comparison failed: %v", compareErr)
	}

	wants := []string{
		"========== COMPARE CONFIG ==========",
		"compareResultSortRows=true",
		"compareResultCaseSensitive=false",
		"compareResultRunConcurrently=true",
		"compareResultConcurrentWorkerCount=7",
		"compareResultConcurrentRunDuration=2m0s",
		"compareResultConcurrentRunsPerPair=11",
		"compareResultReadFromFiles=true",
		"compareResultDataSource=files",
		fmt.Sprintf("compareResultExpectedFile=%q", expectedFile),
		fmt.Sprintf("compareResultActualFile=%q", actualFile),
		`compareResultDBConfig.address="127.0.0.1"`,
		`compareResultDBConfig.port="4000"`,
		`compareResultDBConfig.user="tester"`,
		`compareResultDBConfig.dbName="compare_db"`,
		`compareResultDBConfig.params=["charset=utf8mb4" "timeout=1s"]`,
		"compareResultSQLPairs.count=1",
		`compareResultSQLPairs[0].name="config output case"`,
		"compareResultSQLPairs[0].expected.setupSQLs.count=2",
		`compareResultSQLPairs[0].expected.setupSQLs[0]="use expected_db"`,
		`compareResultSQLPairs[0].expected.setupSQLs[1]="set x = 1"`,
		`compareResultSQLPairs[0].expected.query="select expected_value\nfrom t"`,
		"compareResultSQLPairs[0].actual.setupSQLs.count=1",
		`compareResultSQLPairs[0].actual.setupSQLs[0]="use actual_db"`,
		`compareResultSQLPairs[0].actual.query="select actual_value from t"`,
		"========== END COMPARE CONFIG ==========",
	}
	for _, want := range wants {
		if !strings.Contains(output, want) {
			t.Fatalf("compare output does not contain %q:\n%s", want, output)
		}
	}
	configEnd := strings.Index(output, "========== END COMPARE CONFIG ==========")
	comparisonStart := strings.Index(output, "Reading expected result file")
	if configEnd < 0 || comparisonStart < 0 || configEnd > comparisonStart {
		t.Fatalf("config was not printed before comparison started:\n%s", output)
	}
}

func TestCompareOneSQLPairPrintsStageLogsAndTimingsOnSuccess(t *testing.T) {
	state := &runSQLTestDriverState{}
	driverName := fmt.Sprintf("compare-log-test-%d", time.Now().UnixNano())
	sql.Register(driverName, &runSQLTestDriver{state: state})
	db, err := sql.Open(driverName, "")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	pair := compareSQLPair{
		name: "timed case",
		expected: compareSQL{
			setupSQLs: []string{"set mode = 'expected'"},
			query:     "select expected_value from t",
		},
		actual: compareSQL{
			setupSQLs: []string{"set mode = 'actual'"},
			query:     "select actual_value from t",
		},
	}
	var compareErr error
	output := captureStdout(t, func() {
		compareErr = compareOneSQLPairWithLabel(context.Background(), db, pair, "SQL pair 1 (timed case)")
	})
	if compareErr != nil {
		t.Fatalf("comparison failed: %v", compareErr)
	}
	for _, want := range []string{
		`[SQL pair 1 (timed case)] Executing expected SQL: "select expected_value from t"`,
		`[SQL pair 1 (timed case)] Executing actual SQL: "select actual_value from t"`,
		"[SQL pair 1 (timed case)] Comparing expected and actual result sets",
		"[SQL pair 1 (timed case)] Finished: status=success, total=",
		", expected=",
		", actual=",
		", comparison=",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("comparison output does not contain %q:\n%s", want, output)
		}
	}
	assertCompareStageStartTimes(t, output, []string{
		"Executing expected SQL:",
		"Executing actual SQL:",
		"Comparing expected and actual result sets",
	})
}

func TestCompareOneSQLPairPrintsTimingsOnFailure(t *testing.T) {
	pair := compareSQLPair{
		name: "failing timed case",
		expected: compareSQL{
			query: "",
		},
		actual: compareSQL{
			query: "select actual_value from t",
		},
	}
	var compareErr error
	output := captureStdout(t, func() {
		// An empty expected query fails before the nil DB is accessed.
		compareErr = compareOneSQLPairWithLabel(context.Background(), nil, pair, "SQL pair 1 (failing timed case)")
	})
	if compareErr == nil {
		t.Fatal("comparison unexpectedly succeeded")
	}
	for _, want := range []string{
		`[SQL pair 1 (failing timed case)] Executing expected SQL: ""`,
		"[SQL pair 1 (failing timed case)] Finished: status=failure, total=",
		", expected=",
		", actual=0s, comparison=0s",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("failure output does not contain %q:\n%s", want, output)
		}
	}
}

func TestSummarizeCompareSQLForLogLimitsUnicodeCharacters(t *testing.T) {
	shortSQL := "select 1"
	if got := summarizeCompareSQLForLog(shortSQL); got != shortSQL {
		t.Fatalf("short SQL summary = %q, want %q", got, shortSQL)
	}

	longSQL := strings.Repeat("查", 101)
	got := summarizeCompareSQLForLog(longSQL)
	if runeCount := utf8.RuneCountInString(got); runeCount != compareSQLLogMaxCharacters {
		t.Fatalf("SQL summary has %d characters, want %d", runeCount, compareSQLLogMaxCharacters)
	}
	if !strings.HasSuffix(got, "...") {
		t.Fatalf("truncated SQL summary does not end with ...: %q", got)
	}
	if strings.Contains(got, strings.Repeat("查", 98)) {
		t.Fatalf("SQL summary retained more than 97 content characters: %q", got)
	}

	multilineSQL := "select  *\nfrom\tt"
	if got := summarizeCompareSQLForLog(multilineSQL); got != "select * from t" {
		t.Fatalf("multiline SQL summary = %q, want %q", got, "select * from t")
	}
}

func assertCompareStageStartTimes(t *testing.T, output string, stageMarkers []string) {
	t.Helper()
	lines := strings.Split(output, "\n")
	for _, stageMarker := range stageMarkers {
		var timestamp string
		for _, line := range lines {
			if !strings.Contains(line, stageMarker) {
				continue
			}
			startIndex := strings.Index(line, "start_time=")
			if startIndex >= 0 {
				timestamp = line[startIndex+len("start_time="):]
				break
			}
		}
		if timestamp == "" {
			t.Fatalf("stage %q has no start_time in output:\n%s", stageMarker, output)
		}
		if _, err := time.Parse(time.RFC3339Nano, timestamp); err != nil {
			t.Fatalf("stage %q has invalid start_time %q: %v", stageMarker, timestamp, err)
		}
	}
}

func TestCompareDatabaseCLIOverrides(t *testing.T) {
	options, err := parseCommandLineOptions([]string{
		"--task", "compare-results",
		"--address", "192.0.2.10",
		"--port", "4406",
		"--user", "cli-user",
		"--dbName", "cli-db",
	})
	if err != nil {
		t.Fatal(err)
	}
	if options.taskName != compareResultsTaskName {
		t.Fatalf("task name = %q, want %q", options.taskName, compareResultsTaskName)
	}

	originalConfig := compareResultDBConfig
	compareResultDBConfig = dbConfig{
		address: "hard-coded-address",
		port:    "hard-coded-port",
		user:    "hard-coded-user",
		dbName:  "hard-coded-db",
	}
	t.Cleanup(func() {
		compareResultDBConfig = originalConfig
	})
	applyDBCLIOverrides(&compareResultDBConfig, options.dbOverrides)
	if compareResultDBConfig.address != "192.0.2.10" ||
		compareResultDBConfig.port != "4406" ||
		compareResultDBConfig.user != "cli-user" ||
		compareResultDBConfig.dbName != "cli-db" {
		t.Fatalf("CLI overrides were not applied: %+v", compareResultDBConfig)
	}

	withoutOverrides, err := parseCommandLineOptions([]string{"--task", "compare-results"})
	if err != nil {
		t.Fatal(err)
	}
	before := compareResultDBConfig
	applyDBCLIOverrides(&compareResultDBConfig, withoutOverrides.dbOverrides)
	if !reflect.DeepEqual(compareResultDBConfig, before) {
		t.Fatalf("hard-coded config changed without CLI overrides: got %+v, want %+v", compareResultDBConfig, before)
	}
}

func TestDatabaseCLIOverrideValidation(t *testing.T) {
	positionalOptions, err := parseCommandLineOptions([]string{
		"compare-results",
		"--address", "198.51.100.20",
		"--port", "4001",
	})
	if err != nil {
		t.Fatal(err)
	}
	if positionalOptions.taskName != compareResultsTaskName ||
		positionalOptions.dbOverrides.address.value != "198.51.100.20" ||
		positionalOptions.dbOverrides.port.value != "4001" {
		t.Fatalf("positional task with flags was not parsed: %+v", positionalOptions)
	}

	if _, err := parseCommandLineOptions([]string{
		"--task", "compare-results",
		"--db-name", "removed-alias",
	}); err == nil || !strings.Contains(err.Error(), "flag provided but not defined") {
		t.Fatalf("unexpected removed --db-name behavior: %v", err)
	}

	insertOptions, err := parseCommandLineOptions([]string{
		"--task", "insert",
		"--address", "192.0.2.10",
		"--dbName", "tpcds10",
	})
	if err != nil {
		t.Fatal(err)
	}
	insertConfig := dbConfig{address: "hard-coded-address", dbName: "hard-coded-db"}
	applyDBCLIOverrides(&insertConfig, insertOptions.dbOverrides)
	if insertConfig.address != "192.0.2.10" || insertConfig.dbName != "tpcds10" {
		t.Fatalf("insert CLI overrides were not applied: %+v", insertConfig)
	}
}

func TestRunnerExecutesTasksSeriallyInOrder(t *testing.T) {
	order := make([]int, 0, 3)
	tasks := []func(){
		func() { order = append(order, 1) },
		func() { order = append(order, 2) },
		func() { order = append(order, 3) },
	}
	newRunner(tasks).run()

	if !reflect.DeepEqual(order, []int{1, 2, 3}) {
		t.Fatalf("runner task order = %v, want [1 2 3]", order)
	}
}

func TestRunSQLPairsRandomlyHonorsPerPairLimit(t *testing.T) {
	setConcurrentCompareConfigForTest(t, 6, 0, 25)
	pairs := []compareSQLPair{
		{name: "first"},
		{name: "second"},
		{name: "third"},
	}
	completed := make([]atomic.Int64, len(pairs))

	err := runSQLPairsRandomly(
		context.Background(),
		pairs,
		func(_ context.Context, pairIndex int, _ compareSQLPair) error {
			completed[pairIndex].Add(1)
			return nil
		},
	)
	if err != nil {
		t.Fatalf("random comparison failed: %v", err)
	}
	for i := range pairs {
		if got := completed[i].Load(); got != 25 {
			t.Fatalf("pair %d completed %d runs, want 25", i, got)
		}
	}
}

func TestRunSQLPairsRandomlyHonorsDurationLimit(t *testing.T) {
	const workerCount = 3
	setConcurrentCompareConfigForTest(t, workerCount, 100*time.Millisecond, 0)
	pairs := []compareSQLPair{{name: "duration case"}}
	var started atomic.Int64
	startTime := time.Now()

	err := runSQLPairsRandomly(
		context.Background(),
		pairs,
		func(ctx context.Context, _ int, _ compareSQLPair) error {
			started.Add(1)
			select {
			case <-time.After(10 * time.Millisecond):
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		},
	)
	if err != nil {
		t.Fatalf("duration-limited comparison failed: %v", err)
	}
	if got := started.Load(); got < workerCount {
		t.Fatalf("started comparison count = %d, want at least one per worker (%d)", got, workerCount)
	}
	if elapsed := time.Since(startTime); elapsed < 100*time.Millisecond || elapsed > time.Second {
		t.Fatalf("duration-limited comparison ran for %s, want between 100ms and 1s", elapsed)
	}
}

func TestRunSQLPairsRandomlyStopsOnFirstFailure(t *testing.T) {
	setConcurrentCompareConfigForTest(t, 4, 0, 100)
	pairs := []compareSQLPair{{name: "failing case"}}
	wantErr := errors.New("different result")
	var attempts atomic.Int64

	err := runSQLPairsRandomly(
		context.Background(),
		pairs,
		func(_ context.Context, _ int, _ compareSQLPair) error {
			attempts.Add(1)
			return wantErr
		},
	)
	if !errors.Is(err, wantErr) {
		t.Fatalf("random comparison error = %v, want %v", err, wantErr)
	}
	if got := attempts.Load(); got < 1 || got > 4 {
		t.Fatalf("attempt count after first failure = %d, want between 1 and worker count", got)
	}
}

func TestRunSQLPairsRandomlyRequiresAStopLimit(t *testing.T) {
	setConcurrentCompareConfigForTest(t, 1, 0, 0)
	err := runSQLPairsRandomly(
		context.Background(),
		[]compareSQLPair{{name: "case"}},
		func(context.Context, int, compareSQLPair) error { return nil },
	)
	if err == nil || !strings.Contains(err.Error(), "requires a run-duration or runs-per-pair limit") {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestRunSQLPairsRandomlyPrintsProgress(t *testing.T) {
	setConcurrentCompareConfigForTest(t, 1, 0, 1)
	pairs := []compareSQLPair{{name: "logged case"}}
	var runErr error
	output := captureStdout(t, func() {
		runErr = runSQLPairsRandomly(
			context.Background(),
			pairs,
			func(context.Context, int, compareSQLPair) error { return nil },
		)
	})
	if runErr != nil {
		t.Fatalf("random comparison failed: %v", runErr)
	}
	for _, want := range []string{
		"[worker 1] Start SQL pair 1 (logged case), run 1",
		"[worker 1] Success SQL pair 1 (logged case), run 1",
		"successful=1",
		"Success: concurrent random comparison finished",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("output does not contain %q:\n%s", want, output)
		}
	}
}

func captureStdout(t *testing.T, run func()) string {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	originalStdout := os.Stdout
	os.Stdout = writer
	defer func() {
		os.Stdout = originalStdout
		_ = writer.Close()
		_ = reader.Close()
	}()
	run()
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	os.Stdout = originalStdout
	output, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	return string(output)
}

func setConcurrentCompareConfigForTest(t *testing.T, workers int, duration time.Duration, runsPerPair int) {
	t.Helper()
	originalWorkers := compareResultConcurrentWorkerCount
	originalDuration := compareResultConcurrentRunDuration
	originalRunsPerPair := compareResultConcurrentRunsPerPair
	compareResultConcurrentWorkerCount = workers
	compareResultConcurrentRunDuration = duration
	compareResultConcurrentRunsPerPair = runsPerPair
	t.Cleanup(func() {
		compareResultConcurrentWorkerCount = originalWorkers
		compareResultConcurrentRunDuration = originalDuration
		compareResultConcurrentRunsPerPair = originalRunsPerPair
	})
}

func TestParseTaskName(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		want    string
		wantErr string
	}{
		{name: "insert flag", args: []string{"--task", "insert"}, want: insertTaskName},
		{name: "compare flag", args: []string{"--task=compare-results"}, want: compareResultsTaskName},
		{name: "compare positional alias", args: []string{"compare_result"}, want: compareResultsTaskName},
		{name: "missing", wantErr: "please choose a task"},
		{name: "unknown", args: []string{"unknown"}, wantErr: "unsupported task"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseTaskName(test.args)
			if test.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantErr) {
					t.Fatalf("parseTaskName() error = %v, want substring %q", err, test.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseTaskName() error = %v", err)
			}
			if got != test.want {
				t.Fatalf("parseTaskName() = %q, want %q", got, test.want)
			}
		})
	}
}
