package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

// ------------------------ RUN SQL CONFIG ------------------------

var (
	runSQLDBConfig = dbConfig{
		address: defaultAddress,
		port:    defaultPort,
		user:    defaultUser,
		dbName:  defaultDBName,
		params:  []string{},
	}

	// Workers randomly choose cases from this slice. Every case owns its setup
	// SQLs and query; they always execute on the same database session.
	runSQLCases = []runSQLCase{
		// {
		// 	setupSQLs: []string{
		// 		`use test`,
		// 		`set tidb_enable_chunk_rpc = on`,
		// 	},
		// 	query: `select * from t1 limit 100`,
		// },
	}

	// YAML cases are appended to the hard-coded runSQLCases during task
	// initialization.
	runSQLCasesYAMLFile = "runSqlsCases.yaml"

	// When true, SQL execution errors are logged and the workload continues.
	// When false, the first SQL error cancels all concurrent workers and is
	// returned as the task error.
	runSQLIgnoreErrors = false

	// Number of SQL executions that may be in progress at the same time.
	runSQLConcurrentWorkerCount = 5

	// Zero disables the duration limit.
	runSQLRunDuration = 20 * time.Minute

	// Zero disables the per-statement limit. When positive, every SQL can be
	// scheduled at most this many times. If both limits are enabled, new work
	// stops when the duration elapses or every SQL reaches this limit, whichever
	// happens first. SQL already in progress is allowed to finish.
	runSQLRunsPerStatement = 0
)

// ----------------------------------------------------------------

const (
	runSQLErrorMaxRunes   = 100
	runSQLPreviewMaxRunes = 120
)

type runSQLCase struct {
	setupSQLs []string
	query     string
}

type runSQLCaseYAML struct {
	SetupSQLs []string `yaml:"setupSQLs"`
	Query     string   `yaml:"query"`
}

type runSQLExecutor func(context.Context, int, runSQLCase) (int64, error)

var (
	runSQLCasesYAMLOnce sync.Once
	runSQLCasesYAMLErr  error
)

type runSQLRandomScheduler struct {
	mu                    sync.Mutex
	random                *rand.Rand
	runsPerStatementLimit int
	runCounts             []int
	availableStatements   []int
}

func newRunSQLRandomScheduler(statementCount, runsPerStatementLimit int) *runSQLRandomScheduler {
	availableStatements := make([]int, statementCount)
	for i := range availableStatements {
		availableStatements[i] = i
	}
	return &runSQLRandomScheduler{
		random:                rand.New(rand.NewSource(time.Now().UnixNano())),
		runsPerStatementLimit: runsPerStatementLimit,
		runCounts:             make([]int, statementCount),
		availableStatements:   availableStatements,
	}
}

func (s *runSQLRandomScheduler) next() (statementIndex, runNumber int, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.availableStatements) == 0 {
		return 0, 0, false
	}
	availableIndex := s.random.Intn(len(s.availableStatements))
	statementIndex = s.availableStatements[availableIndex]
	s.runCounts[statementIndex]++
	runNumber = s.runCounts[statementIndex]

	if s.runsPerStatementLimit > 0 && runNumber >= s.runsPerStatementLimit {
		last := len(s.availableStatements) - 1
		s.availableStatements[availableIndex] = s.availableStatements[last]
		s.availableStatements = s.availableStatements[:last]
	}
	return statementIndex, runNumber, true
}

func (s *runSQLRandomScheduler) scheduledRunCounts() []int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]int(nil), s.runCounts...)
}

// runConfiguredSQLs packages the SQL workload as one task. runner remains
// single-threaded; concurrency is owned by the task in runSQLWorkload.
func runConfiguredSQLs() error {
	var runErr error
	task := func() {
		runErr = runConfiguredSQLsTask(context.Background())
	}
	newRunner([]func(){task}).run()
	return runErr
}

func runConfiguredSQLsTask(ctx context.Context) error {
	if err := initializeRunSQLCasesFromYAML(); err != nil {
		return err
	}
	cases := cloneRunSQLCases(runSQLCases)
	if err := validateRunSQLConfig(cases); err != nil {
		return err
	}

	db, err := getDB(runSQLDBConfig)
	if err != nil {
		return fmt.Errorf("connect to run-sqls database: %w", err)
	}
	defer db.Close()

	return runSQLWorkload(ctx, cases, func(ctx context.Context, _ int, configuredSQL runSQLCase) (int64, error) {
		return executeRunSQL(ctx, db, configuredSQL)
	})
}

func initializeRunSQLCasesFromYAML() error {
	runSQLCasesYAMLOnce.Do(func() {
		cases, err := loadRunSQLCasesFromYAML(runSQLCasesYAMLFile)
		if err != nil {
			runSQLCasesYAMLErr = fmt.Errorf("initialize run-sqls cases from YAML: %w", err)
			return
		}
		runSQLCases = append(runSQLCases, cases...)
	})
	return runSQLCasesYAMLErr
}

func loadRunSQLCasesFromYAML(fileName string) ([]runSQLCase, error) {
	if strings.TrimSpace(fileName) == "" {
		return nil, errors.New("run-sqls cases YAML file name cannot be empty")
	}

	file, err := os.Open(fileName)
	if err != nil {
		return nil, fmt.Errorf("open %q: %w", fileName, err)
	}
	defer file.Close()

	decoder := yaml.NewDecoder(file)
	decoder.KnownFields(true)
	var configuredCases []runSQLCaseYAML
	if err := decoder.Decode(&configuredCases); err != nil {
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

	cases := make([]runSQLCase, 0, len(configuredCases))
	for i, configuredCase := range configuredCases {
		if strings.TrimSpace(configuredCase.Query) == "" {
			return nil, fmt.Errorf("%s: case %d query cannot be empty", fileName, i+1)
		}
		cases = append(cases, runSQLCase{
			setupSQLs: append([]string(nil), configuredCase.SetupSQLs...),
			query:     configuredCase.Query,
		})
	}
	return cases, nil
}

func cloneRunSQLCases(cases []runSQLCase) []runSQLCase {
	cloned := make([]runSQLCase, len(cases))
	copy(cloned, cases)
	for i := range cloned {
		cloned[i].setupSQLs = append([]string(nil), cases[i].setupSQLs...)
	}
	return cloned
}

func validateRunSQLConfig(cases []runSQLCase) error {
	if len(cases) == 0 {
		return errors.New("runSQLCases must contain at least one SQL case")
	}
	for i, configuredSQL := range cases {
		if strings.TrimSpace(configuredSQL.query) == "" {
			return fmt.Errorf("runSQLCases[%d].query cannot be empty", i)
		}
	}
	if runSQLConcurrentWorkerCount <= 0 {
		return errors.New("runSQLConcurrentWorkerCount must be greater than zero")
	}
	if runSQLRunDuration < 0 {
		return errors.New("runSQLRunDuration cannot be negative")
	}
	if runSQLRunsPerStatement < 0 {
		return errors.New("runSQLRunsPerStatement cannot be negative")
	}
	if runSQLRunDuration == 0 && runSQLRunsPerStatement == 0 {
		return errors.New("run-sqls requires a duration or runs-per-statement limit")
	}
	return nil
}

func runSQLWorkload(ctx context.Context, cases []runSQLCase, execute runSQLExecutor) error {
	if err := validateRunSQLConfig(cases); err != nil {
		return err
	}
	if execute == nil {
		return errors.New("run-sqls requires an SQL executor")
	}

	workerCount := runSQLConcurrentWorkerCount
	runDuration := runSQLRunDuration
	runsPerStatement := runSQLRunsPerStatement
	limits := make([]string, 0, 3)
	if runDuration > 0 {
		limits = append(limits, fmt.Sprintf("duration=%s", runDuration))
	}
	if runsPerStatement > 0 {
		limits = append(limits, fmt.Sprintf("runs-per-SQL=%d", runsPerStatement))
	}
	limits = append(limits, fmt.Sprintf("ignore-errors=%t", runSQLIgnoreErrors))
	fmt.Printf(
		"Start random SQL workload: workers=%d, SQLs=%d, %s\n",
		workerCount,
		len(cases),
		strings.Join(limits, ", "),
	)

	schedulingBaseCtx, stopScheduling := context.WithCancel(ctx)
	defer stopScheduling()
	schedulingCtx := schedulingBaseCtx
	if runDuration > 0 {
		var stopTimer context.CancelFunc
		schedulingCtx, stopTimer = context.WithTimeout(schedulingBaseCtx, runDuration)
		defer stopTimer()
	}

	scheduler := newRunSQLRandomScheduler(len(cases), runsPerStatement)
	succeededRuns := make([]atomic.Int64, len(cases))
	failedRuns := make([]atomic.Int64, len(cases))
	returnedRows := make([]atomic.Int64, len(cases))
	var totalFailures atomic.Int64
	var firstFailure error
	var failureOnce sync.Once
	var outputMu sync.Mutex
	writeLog := func(format string, args ...any) {
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

				statementIndex, runNumber, ok := scheduler.next()
				if !ok {
					return
				}
				configuredSQL := cases[statementIndex]
				label := fmt.Sprintf("SQL %d, run %d", statementIndex+1, runNumber)
				startedAt := time.Now()
				writeLog(
					"[worker %d] Start %s: %s",
					workerID,
					label,
					summarizeRunSQL(configuredSQL.query),
				)

				// Internal SQL failures stop only future scheduling. An execution
				// already in progress keeps the caller's context and may finish
				// naturally. External cancellation through ctx still interrupts it.
				rowCount, err := execute(ctx, statementIndex, configuredSQL)
				elapsed := time.Since(startedAt)
				if err != nil {
					if ctx.Err() != nil {
						writeLog(
							"[worker %d] Stopped %s, rows=%d, elapsed=%s, error=%s",
							workerID,
							label,
							rowCount,
							elapsed,
							truncateRunSQLError(err),
						)
						return
					}
					failed := failedRuns[statementIndex].Add(1)
					totalFailures.Add(1)
					returnedRows[statementIndex].Add(rowCount)
					writeLog(
						"[worker %d] Failed %s, rows=%d, elapsed=%s, failed=%d, error=%s",
						workerID,
						label,
						rowCount,
						elapsed,
						failed,
						truncateRunSQLError(err),
					)
					if !runSQLIgnoreErrors {
						failureOnce.Do(func() {
							firstFailure = fmt.Errorf("%s: %s", label, truncateRunSQLError(err))
							stopScheduling()
						})
						return
					}
					continue
				}

				succeeded := succeededRuns[statementIndex].Add(1)
				returnedRows[statementIndex].Add(rowCount)
				writeLog(
					"[worker %d] Success %s, rows=%d, elapsed=%s, successful=%d",
					workerID,
					label,
					rowCount,
					elapsed,
					succeeded,
				)
			}
		}(workerIndex + 1)
	}
	wg.Wait()

	scheduledRuns := scheduler.scheduledRunCounts()
	for i, configuredSQL := range cases {
		fmt.Printf(
			"Completed SQL %d: scheduled=%d, successful=%d, failed=%d, rows=%d, statement=%s\n",
			i+1,
			scheduledRuns[i],
			succeededRuns[i].Load(),
			failedRuns[i].Load(),
			returnedRows[i].Load(),
			summarizeRunSQL(configuredSQL.query),
		)
	}

	if ctx.Err() != nil {
		return fmt.Errorf("run-sqls canceled: %w", ctx.Err())
	}
	if firstFailure != nil {
		fmt.Printf("Failed: random SQL workload: %v\n", firstFailure)
		return firstFailure
	}
	if failures := totalFailures.Load(); failures > 0 {
		fmt.Printf("Success: random SQL workload finished with %d ignored error(s)\n", failures)
		return nil
	}
	fmt.Println("Success: random SQL workload finished")
	return nil
}

func executeRunSQL(ctx context.Context, db *sql.DB, configuredSQL runSQLCase) (int64, error) {
	conn, err := db.Conn(ctx)
	if err != nil {
		return 0, fmt.Errorf("acquire database connection: %w", err)
	}
	defer conn.Close()

	for i, setupStatement := range configuredSQL.setupSQLs {
		if strings.TrimSpace(setupStatement) == "" {
			continue
		}
		if _, err := conn.ExecContext(ctx, setupStatement); err != nil {
			return 0, fmt.Errorf("execute setup SQL %d: %w", i+1, err)
		}
	}

	rows, err := conn.QueryContext(ctx, configuredSQL.query)
	if err != nil {
		return 0, fmt.Errorf("execute SQL: %w", err)
	}
	defer rows.Close()

	var rowCount int64
	for {
		for rows.Next() {
			rowCount++
		}
		if err := rows.Err(); err != nil {
			return rowCount, fmt.Errorf("read returned rows: %w", err)
		}
		if !rows.NextResultSet() {
			if err := rows.Err(); err != nil {
				return rowCount, fmt.Errorf("advance returned result sets: %w", err)
			}
			break
		}
	}
	if err := rows.Close(); err != nil {
		return rowCount, fmt.Errorf("close returned rows: %w", err)
	}
	return rowCount, nil
}

func truncateRunSQLError(err error) string {
	if err == nil {
		return ""
	}
	message := strings.Join(strings.Fields(err.Error()), " ")
	return truncateRunes(message, runSQLErrorMaxRunes)
}

func summarizeRunSQL(statement string) string {
	statement = strings.Join(strings.Fields(statement), " ")
	return truncateRunes(statement, runSQLPreviewMaxRunes)
}

func truncateRunes(value string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	if utf8.RuneCountInString(value) <= maxRunes {
		return value
	}
	if maxRunes <= 3 {
		return strings.Repeat(".", maxRunes)
	}
	runes := []rune(value)
	return string(runes[:maxRunes-3]) + "..."
}
