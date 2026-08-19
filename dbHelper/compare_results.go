package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"
	"unicode/utf8"

	_ "github.com/go-sql-driver/mysql"
	"gopkg.in/yaml.v3"
)

// ---------------------- COMPARE CONFIG ----------------------

var (
	// compareResultSortRows controls whether rows are sorted before comparison.
	compareResultSortRows = true

	// compareResultCaseSensitive controls whether text comparison is case-sensitive.
	compareResultCaseSensitive = true

	// compareResultRunConcurrently controls whether multiple SQL pairs are checked
	// concurrently. It has no effect in file mode, because file mode always compares
	// exactly the two files configured below.
	compareResultRunConcurrently = false

	// The following three variables are used only when
	// compareResultRunConcurrently is true. Workers repeatedly choose a random
	// pair from compareResultSQLPairs instead of checking every pair just once.
	compareResultConcurrentWorkerCount = 4

	// A zero duration disables the time limit.
	compareResultConcurrentRunDuration = 1 * time.Minute

	// Zero disables the per-pair limit. When this value is positive, every pair
	// can be scheduled at most this many times. If both limits are enabled, new
	// work stops when the duration elapses or every pair reaches its run limit,
	// whichever happens first. Comparisons already in progress are allowed to
	// finish.
	compareResultConcurrentRunsPerPair = 0

	// compareResultReadFromFiles selects the source of the two result sets:
	// true reads the two files below; false executes compareResultSQLPairs.
	compareResultReadFromFiles = false

	compareResultExpectedFile = ""
	compareResultActualFile   = ""

	// Cases loaded from this YAML file are appended to the hard-coded
	// compareResultSQLPairs before SQL comparison starts.
	compareResultCasesYAMLFile = "compareCases.yaml"

	// You can override it with parameter in CLI
	// For example: go run . --task compare-results --addr
	compareResultDBConfig = dbConfig{
		address: defaultAddress,
		port:    defaultPort,
		user:    defaultUser,
		dbName:  defaultDBName,
		params:  []string{},
	}

	// Add as many pairs as needed. setupSQLs are executed on the same database
	// session as query, immediately before query is executed.
	//
	// Example:
	// {
	//     name: "partial ordered index for topn",
	//     expected: compareSQL{
	//         setupSQLs: []string{`set tidb_opt_partial_ordered_index_for_topn="disable"`},
	//         query:     "select c0, c1 from t1",
	//     },
	//     actual: compareSQL{
	//         setupSQLs: []string{`set tidb_opt_partial_ordered_index_for_topn="cost"`},
	//         query:     "select c0, c1 from t1",
	//     },
	// },
	//
	// {
	// 	name: "",
	// 	expected: compareSQL{
	// 		setupSQLs: []string{
	// 			"",
	// 		},
	// 		query: "",
	// 	},
	// 	actual: compareSQL{
	// 		setupSQLs: []string{
	// 			"",
	// 		},
	// 		query: "",
	// 	},
	// },
	//
	// Moreover, you can config it in the `compareCases.yaml` file.
	compareResultSQLPairs = []compareSQLPair{}
)

// ------------------------------------------------------------

type compareSQL struct {
	setupSQLs []string
	query     string
}

type compareSQLPair struct {
	name     string
	expected compareSQL
	actual   compareSQL
}

type compareSQLYAML struct {
	SetupSQLs []string `yaml:"setupSQLs"`
	Query     string   `yaml:"query"`
}

type compareSQLPairYAML struct {
	Name     string         `yaml:"name"`
	Expected compareSQLYAML `yaml:"expected"`
	Actual   compareSQLYAML `yaml:"actual"`
}

type compareCell struct {
	value  string
	isNull bool
}

type compareRow []compareCell

const compareSQLLogMaxCharacters = 100

type compareStageTimings struct {
	totalStart         time.Time
	expectedDuration   time.Duration
	actualDuration     time.Duration
	comparisonDuration time.Duration
}

var (
	compareLogMu         sync.Mutex
	compareFailureFileMu sync.Mutex
	compareCasesYAMLOnce sync.Once
	compareCasesYAMLErr  error
)

func writeCompareLog(format string, args ...any) {
	compareLogMu.Lock()
	defer compareLogMu.Unlock()
	fmt.Printf(format+"\n", args...)
}

func (timings compareStageTimings) printSummary(label string, resultErr error) {
	status := "success"
	if resultErr != nil {
		status = "failure"
	}
	writeCompareLog(
		"[%s] Finished: status=%s, total=%s, expected=%s, actual=%s, comparison=%s",
		label,
		status,
		time.Since(timings.totalStart),
		timings.expectedDuration,
		timings.actualDuration,
		timings.comparisonDuration,
	)
}

// runCompareResults packages the complete comparison as one task and hands it
// to runner. runner itself is deliberately single-threaded; optional SQL-pair
// concurrency is implemented inside the task.
func runCompareResults() error {
	var compareErr error
	task := func() {
		compareErr = compareResultsTask(context.Background())
	}
	newRunner([]func(){task}).run()
	return compareErr
}

func compareResultsTask(ctx context.Context) error {
	if !compareResultReadFromFiles {
		if err := initializeCompareResultSQLPairsFromYAML(); err != nil {
			return err
		}
	}
	printCompareResultConfig()
	if compareResultReadFromFiles {
		return compareConfiguredFiles()
	}
	return compareConfiguredSQLPairs(ctx)
}

func printCompareResultConfig() {
	dataSource := "sql"
	if compareResultReadFromFiles {
		dataSource = "files"
	}

	fmt.Println("========== COMPARE CONFIG ==========")
	fmt.Printf("compareResultSortRows=%t\n", compareResultSortRows)
	fmt.Printf("compareResultCaseSensitive=%t\n", compareResultCaseSensitive)
	fmt.Printf("compareResultRunConcurrently=%t\n", compareResultRunConcurrently)
	fmt.Printf("compareResultConcurrentWorkerCount=%d\n", compareResultConcurrentWorkerCount)
	fmt.Printf("compareResultConcurrentRunDuration=%s\n", compareResultConcurrentRunDuration)
	fmt.Printf("compareResultConcurrentRunsPerPair=%d\n", compareResultConcurrentRunsPerPair)
	fmt.Printf("compareResultReadFromFiles=%t\n", compareResultReadFromFiles)
	fmt.Printf("compareResultDataSource=%s\n", dataSource)
	fmt.Printf("compareResultExpectedFile=%q\n", compareResultExpectedFile)
	fmt.Printf("compareResultActualFile=%q\n", compareResultActualFile)
	fmt.Printf("compareResultCasesYAMLFile=%q\n", compareResultCasesYAMLFile)
	fmt.Printf("compareResultDBConfig.address=%q\n", compareResultDBConfig.address)
	fmt.Printf("compareResultDBConfig.port=%q\n", compareResultDBConfig.port)
	fmt.Printf("compareResultDBConfig.user=%q\n", compareResultDBConfig.user)
	fmt.Printf("compareResultDBConfig.dbName=%q\n", compareResultDBConfig.dbName)
	fmt.Printf("compareResultDBConfig.params=%q\n", compareResultDBConfig.params)
	fmt.Printf("compareResultSQLPairs.count=%d\n", len(compareResultSQLPairs))
	// for pairIndex, pair := range compareResultSQLPairs {
	// 	pairPrefix := fmt.Sprintf("compareResultSQLPairs[%d]", pairIndex)
	// 	fmt.Printf("%s.name=%q\n", pairPrefix, pair.name)
	// 	printCompareSQLConfig(pairPrefix+".expected", pair.expected)
	// 	printCompareSQLConfig(pairPrefix+".actual", pair.actual)
	// }
	fmt.Println("========== END COMPARE CONFIG ==========")
}

func initializeCompareResultSQLPairsFromYAML() error {
	compareCasesYAMLOnce.Do(func() {
		pairs, err := loadCompareSQLPairsFromYAML(compareResultCasesYAMLFile)
		if err != nil {
			compareCasesYAMLErr = fmt.Errorf("initialize compare cases from YAML: %w", err)
			return
		}
		compareResultSQLPairs = append(compareResultSQLPairs, pairs...)
	})
	return compareCasesYAMLErr
}

func loadCompareSQLPairsFromYAML(fileName string) ([]compareSQLPair, error) {
	if strings.TrimSpace(fileName) == "" {
		return nil, errors.New("compare cases YAML file name cannot be empty")
	}

	file, err := os.Open(fileName)
	if err != nil {
		return nil, fmt.Errorf("open %q: %w", fileName, err)
	}
	defer file.Close()

	decoder := yaml.NewDecoder(file)
	decoder.KnownFields(true)
	var configuredPairs []compareSQLPairYAML
	if err := decoder.Decode(&configuredPairs); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, nil
		}
		return nil, fmt.Errorf("decode %q: %w", fileName, err)
	}
	var extraDocument any
	if err := decoder.Decode(&extraDocument); !errors.Is(err, io.EOF) {
		if err == nil {
			return nil, fmt.Errorf("decode %q: multiple YAML documents are not supported", fileName)
		}
		return nil, fmt.Errorf("decode trailing content in %q: %w", fileName, err)
	}

	pairs := make([]compareSQLPair, 0, len(configuredPairs))
	for i, configuredPair := range configuredPairs {
		if strings.TrimSpace(configuredPair.Name) == "" {
			return nil, fmt.Errorf("%s: case %d name cannot be empty", fileName, i+1)
		}
		if strings.TrimSpace(configuredPair.Expected.Query) == "" {
			return nil, fmt.Errorf("%s: case %d (%s) expected.query cannot be empty", fileName, i+1, configuredPair.Name)
		}
		if strings.TrimSpace(configuredPair.Actual.Query) == "" {
			return nil, fmt.Errorf("%s: case %d (%s) actual.query cannot be empty", fileName, i+1, configuredPair.Name)
		}
		pairs = append(pairs, compareSQLPair{
			name: configuredPair.Name,
			expected: compareSQL{
				setupSQLs: append([]string(nil), configuredPair.Expected.SetupSQLs...),
				query:     configuredPair.Expected.Query,
			},
			actual: compareSQL{
				setupSQLs: append([]string(nil), configuredPair.Actual.SetupSQLs...),
				query:     configuredPair.Actual.Query,
			},
		})
	}
	return pairs, nil
}

func printCompareSQLConfig(prefix string, configuredSQL compareSQL) {
	fmt.Printf("%s.setupSQLs.count=%d\n", prefix, len(configuredSQL.setupSQLs))
	for setupIndex, setupSQL := range configuredSQL.setupSQLs {
		fmt.Printf("%s.setupSQLs[%d]=%q\n", prefix, setupIndex, setupSQL)
	}
	fmt.Printf("%s.query=%q\n", prefix, configuredSQL.query)
}

func compareConfiguredFiles() (resultErr error) {
	label := fmt.Sprintf("File comparison %q vs %q", compareResultExpectedFile, compareResultActualFile)
	timings := compareStageTimings{totalStart: time.Now()}
	defer func() {
		timings.printSummary(label, resultErr)
	}()

	if strings.TrimSpace(compareResultExpectedFile) == "" || strings.TrimSpace(compareResultActualFile) == "" {
		return errors.New("file comparison requires both compareResultExpectedFile and compareResultActualFile")
	}

	expectedStart := time.Now()
	writeCompareLog(
		"[%s] Reading expected result file, start_time=%s",
		label,
		expectedStart.Format(time.RFC3339Nano),
	)
	expected, err := readResultFile(compareResultExpectedFile)
	timings.expectedDuration = time.Since(expectedStart)
	if err != nil {
		return fmt.Errorf("read expected result file %q: %w", compareResultExpectedFile, err)
	}
	actualStart := time.Now()
	writeCompareLog(
		"[%s] Reading actual result file, start_time=%s",
		label,
		actualStart.Format(time.RFC3339Nano),
	)
	actual, err := readResultFile(compareResultActualFile)
	timings.actualDuration = time.Since(actualStart)
	if err != nil {
		return fmt.Errorf("read actual result file %q: %w", compareResultActualFile, err)
	}
	comparisonStart := time.Now()
	writeCompareLog(
		"[%s] Comparing expected and actual result sets, start_time=%s",
		label,
		comparisonStart.Format(time.RFC3339Nano),
	)
	if err := compareResultRows(expected, actual); err != nil {
		timings.comparisonDuration = time.Since(comparisonStart)
		return err
	}
	timings.comparisonDuration = time.Since(comparisonStart)
	writeCompareLog("[%s] Success: rows=%d", label, len(expected))
	return nil
}

func compareConfiguredSQLPairs(ctx context.Context) error {
	if len(compareResultSQLPairs) == 0 {
		return errors.New("SQL comparison requires at least one entry in compareResultSQLPairs")
	}
	if compareResultRunConcurrently {
		if err := validateConcurrentCompareConfig(); err != nil {
			return err
		}
	}

	db, err := getDB(compareResultDBConfig)
	if err != nil {
		return fmt.Errorf("connect to comparison database: %w", err)
	}
	defer db.Close()

	// Session variables set by setupSQLs must not leak into another SQL in a
	// later case. With no idle connections, releasing *sql.Conn also closes the
	// underlying connection and therefore its session state.
	db.SetMaxIdleConns(0)

	if compareResultRunConcurrently {
		return compareSQLPairsConcurrently(ctx, db, compareResultSQLPairs)
	}
	return compareSQLPairsSerially(ctx, db, compareResultSQLPairs)
}

func compareSQLPairsSerially(ctx context.Context, db *sql.DB, pairs []compareSQLPair) error {
	for i, pair := range pairs {
		label := sqlPairLabel(i, pair)
		writeCompareLog("[%s] Starting comparison", label)
		if err := compareOneSQLPairWithLabel(ctx, db, pair, label); err != nil {
			return fmt.Errorf("%s: %w", label, err)
		}
		writeCompareLog("[%s] Success", label)
	}
	return nil
}

func compareSQLPairsConcurrently(ctx context.Context, db *sql.DB, pairs []compareSQLPair) error {
	return runSQLPairsRandomly(ctx, pairs, func(ctx context.Context, pairIndex int, pair compareSQLPair) error {
		return compareOneSQLPairWithLabel(ctx, db, pair, sqlPairLabel(pairIndex, pair))
	})
}

type comparePairFunc func(context.Context, int, compareSQLPair) error

// randomPairScheduler randomly chooses one currently available pair. In
// per-pair-count mode, a pair is removed from the available range as soon as
// its configured number of runs has been reserved.
type randomPairScheduler struct {
	mu               sync.Mutex
	random           *rand.Rand
	runsPerPairLimit int
	runCounts        []int
	availablePairs   []int
}

func newRandomPairScheduler(pairCount, runsPerPairLimit int) *randomPairScheduler {
	availablePairs := make([]int, pairCount)
	for i := range availablePairs {
		availablePairs[i] = i
	}
	return &randomPairScheduler{
		random:           rand.New(rand.NewSource(time.Now().UnixNano())),
		runsPerPairLimit: runsPerPairLimit,
		runCounts:        make([]int, pairCount),
		availablePairs:   availablePairs,
	}
}

func (s *randomPairScheduler) next() (pairIndex, runNumber int, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.availablePairs) == 0 {
		return 0, 0, false
	}
	availableIndex := s.random.Intn(len(s.availablePairs))
	pairIndex = s.availablePairs[availableIndex]
	s.runCounts[pairIndex]++
	runNumber = s.runCounts[pairIndex]

	if s.runsPerPairLimit > 0 && runNumber >= s.runsPerPairLimit {
		last := len(s.availablePairs) - 1
		s.availablePairs[availableIndex] = s.availablePairs[last]
		s.availablePairs = s.availablePairs[:last]
	}
	return pairIndex, runNumber, true
}

func (s *randomPairScheduler) scheduledRunCounts() []int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]int(nil), s.runCounts...)
}

func validateConcurrentCompareConfig() error {
	if compareResultConcurrentWorkerCount <= 0 {
		return errors.New("compareResultConcurrentWorkerCount must be greater than zero")
	}
	if compareResultConcurrentRunDuration < 0 {
		return errors.New("compareResultConcurrentRunDuration cannot be negative")
	}
	if compareResultConcurrentRunsPerPair < 0 {
		return errors.New("compareResultConcurrentRunsPerPair cannot be negative")
	}
	if compareResultConcurrentRunDuration == 0 && compareResultConcurrentRunsPerPair == 0 {
		return errors.New("concurrent comparison requires a run-duration or runs-per-pair limit")
	}
	return nil
}

func runSQLPairsRandomly(ctx context.Context, pairs []compareSQLPair, comparePair comparePairFunc) error {
	if len(pairs) == 0 {
		return errors.New("random SQL comparison requires at least one SQL pair")
	}
	if comparePair == nil {
		return errors.New("random SQL comparison requires a comparison function")
	}
	if err := validateConcurrentCompareConfig(); err != nil {
		return err
	}

	workerCount := compareResultConcurrentWorkerCount
	runDuration := compareResultConcurrentRunDuration
	runsPerPair := compareResultConcurrentRunsPerPair

	limitDescription := make([]string, 0, 2)
	if runDuration > 0 {
		limitDescription = append(limitDescription, fmt.Sprintf("duration=%s", runDuration))
	}
	if runsPerPair > 0 {
		limitDescription = append(limitDescription, fmt.Sprintf("runs-per-pair=%d", runsPerPair))
	}
	fmt.Printf(
		"Start concurrent random comparison: workers=%d, SQL pairs=%d, %s\n",
		workerCount,
		len(pairs),
		strings.Join(limitDescription, ", "),
	)

	workerCtx, stopWorkers := context.WithCancel(ctx)
	defer stopWorkers()
	schedulingCtx := workerCtx
	if runDuration > 0 {
		var stopTimer context.CancelFunc
		schedulingCtx, stopTimer = context.WithTimeout(workerCtx, runDuration)
		defer stopTimer()
	}

	scheduler := newRandomPairScheduler(len(pairs), runsPerPair)
	completedRuns := make([]atomic.Int64, len(pairs))
	var firstFailure error
	var failureOnce sync.Once
	var outputMu sync.Mutex
	writeRunLog := func(format string, args ...any) {
		outputMu.Lock()
		defer outputMu.Unlock()
		fmt.Printf(format+"\n", args...)
	}
	var wg sync.WaitGroup
	wg.Add(workerCount)
	for workerIndex := range workerCount {
		go func(workerID int) {
			defer wg.Done()
			for {
				select {
				case <-schedulingCtx.Done():
					return
				default:
				}

				pairIndex, runNumber, ok := scheduler.next()
				if !ok {
					return
				}

				label := sqlPairLabel(pairIndex, pairs[pairIndex])
				startedAt := time.Now()
				writeRunLog("[worker %d] Start %s, run %d", workerID, label, runNumber)
				if err := comparePair(workerCtx, pairIndex, pairs[pairIndex]); err != nil {
					elapsed := time.Since(startedAt)
					// Other in-flight queries may be interrupted after one worker has
					// already reported the real failure. Do not replace that first error
					// with a secondary context-cancellation error.
					if workerCtx.Err() != nil {
						writeRunLog(
							"[worker %d] Stopped %s, run %d, elapsed=%s, error=%v",
							workerID,
							label,
							runNumber,
							elapsed,
							err,
						)
						return
					}
					writeRunLog(
						"[worker %d] Failed %s, run %d, elapsed=%s, error=%v",
						workerID,
						label,
						runNumber,
						elapsed,
						err,
					)
					failureOnce.Do(func() {
						firstFailure = fmt.Errorf(
							"%s, run %d: %w",
							label,
							runNumber,
							err,
						)
						stopWorkers()
					})
					return
				}
				completed := completedRuns[pairIndex].Add(1)
				writeRunLog(
					"[worker %d] Success %s, run %d, elapsed=%s, successful=%d",
					workerID,
					label,
					runNumber,
					time.Since(startedAt),
					completed,
				)
			}
		}(workerIndex + 1)
	}
	wg.Wait()

	scheduledRuns := scheduler.scheduledRunCounts()
	for i := range pairs {
		fmt.Printf(
			"Completed %s: successful=%d, scheduled=%d\n",
			sqlPairLabel(i, pairs[i]),
			completedRuns[i].Load(),
			scheduledRuns[i],
		)
	}

	if firstFailure != nil {
		return firstFailure
	}
	if ctx.Err() != nil {
		return fmt.Errorf("concurrent comparison canceled: %w", ctx.Err())
	}
	fmt.Println("Success: concurrent random comparison finished")
	return nil
}

func sqlPairLabel(index int, pair compareSQLPair) string {
	if strings.TrimSpace(pair.name) == "" {
		return fmt.Sprintf("SQL pair %d", index+1)
	}
	return fmt.Sprintf("SQL pair %d (%s)", index+1, pair.name)
}

func compareOneSQLPair(ctx context.Context, db *sql.DB, pair compareSQLPair) error {
	label := "SQL pair"
	if strings.TrimSpace(pair.name) != "" {
		label = fmt.Sprintf("SQL pair (%s)", pair.name)
	}
	return compareOneSQLPairWithLabel(ctx, db, pair, label)
}

func compareOneSQLPairWithLabel(ctx context.Context, db *sql.DB, pair compareSQLPair, label string) (resultErr error) {
	timings := compareStageTimings{totalStart: time.Now()}
	defer func() {
		timings.printSummary(label, resultErr)
	}()

	expectedStart := time.Now()
	writeCompareLog(
		"[%s] Executing expected SQL: %q, start_time=%s",
		label,
		summarizeCompareSQLForLog(pair.expected.query),
		expectedStart.Format(time.RFC3339Nano),
	)
	expected, err := executeCompareSQL(ctx, db, pair.expected)
	timings.expectedDuration = time.Since(expectedStart)
	if err != nil {
		return fmt.Errorf("execute expected SQL: %w", err)
	}

	actualStart := time.Now()
	writeCompareLog(
		"[%s] Executing actual SQL: %q, start_time=%s",
		label,
		summarizeCompareSQLForLog(pair.actual.query),
		actualStart.Format(time.RFC3339Nano),
	)
	actual, err := executeCompareSQL(ctx, db, pair.actual)
	timings.actualDuration = time.Since(actualStart)
	if err != nil {
		return fmt.Errorf("execute actual SQL: %w", err)
	}

	comparisonStart := time.Now()
	writeCompareLog(
		"[%s] Comparing expected and actual result sets, start_time=%s",
		label,
		comparisonStart.Format(time.RFC3339Nano),
	)
	resultErr = compareResultRows(expected, actual)
	timings.comparisonDuration = time.Since(comparisonStart)
	if resultErr != nil {
		if writeErr := writeCompareFailureFiles(".", pair, expected, actual); writeErr != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("write comparison failure files: %w", writeErr))
		}
	} else {
		writeCompareLog("[%s] Success: rows=%d", label, len(expected))
	}
	return resultErr
}

func writeCompareFailureFiles(outputDir string, pair compareSQLPair, expected, actual []compareRow) error {
	pairName := strings.TrimSpace(pair.name)
	if pairName == "" {
		return errors.New("compare SQL pair name cannot be empty when writing failure files")
	}

	expectedContent := formatCompareFailureFile(expected, pair.expected)
	actualContent := formatCompareFailureFile(actual, pair.actual)

	compareFailureFileMu.Lock()
	defer compareFailureFileMu.Unlock()
	expectedFile, err := writeUniqueCompareFailureFile(outputDir, pairName, "expected", expectedContent)
	if err != nil {
		return fmt.Errorf("write expected result file: %w", err)
	}
	actualFile, err := writeUniqueCompareFailureFile(outputDir, pairName, "actual", actualContent)
	if err != nil {
		return fmt.Errorf("write actual result file: %w", err)
	}

	writeCompareLog("Wrote mismatched expected result to %q", expectedFile)
	writeCompareLog("Wrote mismatched actual result to %q", actualFile)
	return nil
}

func writeUniqueCompareFailureFile(outputDir, pairName, side, content string) (string, error) {
	for suffix := 0; ; suffix++ {
		fileName := fmt.Sprintf("%s-%s.txt", pairName, side)
		if suffix > 0 {
			fileName = fmt.Sprintf("%s-%s(%d).txt", pairName, side, suffix)
		}
		filePath := filepath.Join(outputDir, fileName)
		file, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if errors.Is(err, os.ErrExist) {
			continue
		}
		if err != nil {
			return "", fmt.Errorf("create %q: %w", filePath, err)
		}

		_, writeErr := file.WriteString(content)
		closeErr := file.Close()
		if writeErr != nil || closeErr != nil {
			if removeErr := os.Remove(filePath); removeErr != nil {
				return "", errors.Join(writeErr, closeErr, fmt.Errorf("remove incomplete file %q: %w", filePath, removeErr))
			}
			return "", errors.Join(writeErr, closeErr)
		}
		return filePath, nil
	}
}

func formatCompareFailureFile(rows []compareRow, configuredSQL compareSQL) string {
	var content strings.Builder
	for _, row := range rows {
		content.WriteString(formatCompareRow(row))
		content.WriteByte('\n')
	}

	content.WriteString("\n===== SETUP SQLS =====\n")
	if len(configuredSQL.setupSQLs) == 0 {
		content.WriteString("(none)\n")
	} else {
		for i, setupSQL := range configuredSQL.setupSQLs {
			fmt.Fprintf(&content, "-- setup SQL %d\n%s\n", i+1, setupSQL)
		}
	}
	content.WriteString("\n===== SQL =====\n")
	content.WriteString(configuredSQL.query)
	content.WriteByte('\n')
	return content.String()
}

func summarizeCompareSQLForLog(query string) string {
	query = strings.Join(strings.Fields(query), " ")
	if utf8.RuneCountInString(query) <= compareSQLLogMaxCharacters {
		return query
	}
	runes := []rune(query)
	return string(runes[:compareSQLLogMaxCharacters-3]) + "..."
}

// executeCompareSQL keeps setupSQLs and query on the same connection. This is
// essential for session-scoped statements such as SET and USE.
func executeCompareSQL(ctx context.Context, db *sql.DB, configuredSQL compareSQL) ([]compareRow, error) {
	if strings.TrimSpace(configuredSQL.query) == "" {
		return nil, errors.New("query cannot be empty")
	}

	conn, err := db.Conn(ctx)
	if err != nil {
		return nil, fmt.Errorf("acquire database connection: %w", err)
	}
	defer conn.Close()

	for i, setupSQL := range configuredSQL.setupSQLs {
		if strings.TrimSpace(setupSQL) == "" {
			continue
		}
		if _, err := conn.ExecContext(ctx, setupSQL); err != nil {
			return nil, fmt.Errorf("execute setup SQL %d (%q): %w", i+1, setupSQL, err)
		}
	}

	rows, err := conn.QueryContext(ctx, configuredSQL.query)
	if err != nil {
		return nil, fmt.Errorf("execute query %q: %w", configuredSQL.query, err)
	}
	defer rows.Close()

	columnNames, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("read result columns: %w", err)
	}
	result := make([]compareRow, 0)
	for rows.Next() {
		values := make([]any, len(columnNames))
		destinations := make([]any, len(columnNames))
		for i := range values {
			destinations[i] = &values[i]
		}
		if err := rows.Scan(destinations...); err != nil {
			return nil, fmt.Errorf("scan result row %d: %w", len(result), err)
		}

		row := make(compareRow, len(values))
		for i, value := range values {
			row[i] = databaseValueToCompareCell(value)
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate result rows: %w", err)
	}
	return result, nil
}

func databaseValueToCompareCell(value any) compareCell {
	switch value := value.(type) {
	case nil:
		return compareCell{isNull: true}
	case []byte:
		return compareCell{value: string(value)}
	case string:
		return compareCell{value: value}
	default:
		return compareCell{value: fmt.Sprint(value)}
	}
}

// readResultFile has the same row boundaries as Python's text-mode readlines:
// line endings are retained and CRLF/CR are normalized to LF.
func readResultFile(fileName string) ([]compareRow, error) {
	data, err := os.ReadFile(fileName)
	if err != nil {
		return nil, err
	}
	if !utf8.Valid(data) {
		return nil, errors.New("result file is not valid UTF-8")
	}

	content := strings.ReplaceAll(string(data), "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")
	if content == "" {
		return []compareRow{}, nil
	}

	lines := strings.SplitAfter(content, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	rows := make([]compareRow, len(lines))
	for i, line := range lines {
		rows[i] = compareRow{{value: line}}
	}
	return rows, nil
}

func compareResultRows(expected, actual []compareRow) error {
	if len(expected) != len(actual) {
		return fmt.Errorf("length not equal, %d vs %d", len(expected), len(actual))
	}

	if compareResultSortRows {
		expected = sortedResultRows(expected)
		actual = sortedResultRows(actual)
	}

	for i := range expected {
		if !compareRowsEqual(expected[i], actual[i]) {
			return fmt.Errorf(
				"Incorrect answer: row %d, <%s> vs <%s>",
				i,
				formatCompareRow(expected[i]),
				formatCompareRow(actual[i]),
			)
		}
	}
	return nil
}

func sortedResultRows(rows []compareRow) []compareRow {
	result := append([]compareRow(nil), rows...)
	sort.SliceStable(result, func(i, j int) bool {
		return compareRowKey(result[i]) < compareRowKey(result[j])
	})
	return result
}

func compareRowsEqual(expected, actual compareRow) bool {
	if len(expected) != len(actual) {
		return false
	}
	for i := range expected {
		if expected[i].isNull != actual[i].isNull {
			return false
		}
		if expected[i].isNull {
			continue
		}
		if normalizeCompareText(expected[i].value) != normalizeCompareText(actual[i].value) {
			return false
		}
	}
	return true
}

func compareRowKey(row compareRow) string {
	var key strings.Builder
	for _, cell := range row {
		if cell.isNull {
			key.WriteString("N;")
			continue
		}
		value := normalizeCompareText(cell.value)
		key.WriteByte('V')
		key.WriteString(strconv.Itoa(len(value)))
		key.WriteByte(':')
		key.WriteString(value)
		key.WriteByte(';')
	}
	return key.String()
}

func normalizeCompareText(value string) string {
	if compareResultCaseSensitive {
		return value
	}

	// Canonicalize every Unicode simple-fold cycle to its smallest rune. This
	// gives both equality checks and sorting one stable key, including cases such
	// as Greek sigma (Σ/σ/ς) that strings.ToLower alone does not fully cover.
	var normalized strings.Builder
	for _, current := range value {
		canonical := current
		for folded := unicode.SimpleFold(current); folded != current; folded = unicode.SimpleFold(folded) {
			if folded < canonical {
				canonical = folded
			}
		}
		normalized.WriteRune(canonical)
	}
	return normalized.String()
}

func formatCompareRow(row compareRow) string {
	values := make([]string, len(row))
	for i, cell := range row {
		if cell.isNull {
			values[i] = "NULL"
			continue
		}
		values[i] = cell.value
	}
	return strings.Join(values, "\t")
}
