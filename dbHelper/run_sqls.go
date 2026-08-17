package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"
)

// ------------------------ RUN SQL CONFIG ------------------------

var (
	runSQLDBConfig = dbConfig{
		address: "10.2.12.124",
		port:    "8001",
		user:    "root",
		dbName:  "distinct_test",
		params:  []string{},
	}

	// Workers randomly choose statements from this slice. Empty statements are
	// rejected during configuration validation.
	runSQLStatements = []string{
		// `select * from t1 limit 100`,
		// `select count(*) from t2`,
	}

	// Every statement below is executed in order before each randomly selected
	// runSQLStatements entry. Setup statements and the selected SQL always use
	// the same database session, so SET and USE take effect as expected.
	runSQLSetupStatements = []string{
		// `use test`,
		// `set tidb_enable_chunk_rpc = on`,
	}

	// Number of SQL executions that may be in progress at the same time.
	runSQLConcurrentWorkerCount = 4

	// Zero disables the duration limit.
	runSQLRunDuration = 10 * time.Minute

	// Zero disables the per-statement limit. When positive, every SQL can be
	// scheduled at most this many times. If both limits are enabled, new work
	// stops when the duration elapses or every SQL reaches this limit, whichever
	// happens first. SQL already in progress is allowed to finish.
	runSQLRunsPerStatement = 0
)

// ----------------------------------------------------------------

const (
	runSQLsTaskName       = "run-sqls"
	runSQLErrorMaxRunes   = 100
	runSQLPreviewMaxRunes = 120
)

type runSQLExecutor func(context.Context, int, string) (int64, error)

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
	statements := append([]string(nil), runSQLStatements...)
	setupStatements := append([]string(nil), runSQLSetupStatements...)
	if err := validateRunSQLConfig(statements); err != nil {
		return err
	}

	db, err := getDB(runSQLDBConfig)
	if err != nil {
		return fmt.Errorf("connect to run-sqls database: %w", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(runSQLConcurrentWorkerCount)
	db.SetMaxIdleConns(runSQLConcurrentWorkerCount)

	return runSQLWorkload(ctx, statements, func(ctx context.Context, _ int, statement string) (int64, error) {
		return executeRunSQL(ctx, db, setupStatements, statement)
	})
}

func validateRunSQLConfig(statements []string) error {
	if len(statements) == 0 {
		return errors.New("runSQLStatements must contain at least one SQL statement")
	}
	for i, statement := range statements {
		if strings.TrimSpace(statement) == "" {
			return fmt.Errorf("runSQLStatements[%d] cannot be empty", i)
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

func runSQLWorkload(ctx context.Context, statements []string, execute runSQLExecutor) error {
	if err := validateRunSQLConfig(statements); err != nil {
		return err
	}
	if execute == nil {
		return errors.New("run-sqls requires an SQL executor")
	}

	workerCount := runSQLConcurrentWorkerCount
	runDuration := runSQLRunDuration
	runsPerStatement := runSQLRunsPerStatement
	limits := make([]string, 0, 2)
	if runDuration > 0 {
		limits = append(limits, fmt.Sprintf("duration=%s", runDuration))
	}
	if runsPerStatement > 0 {
		limits = append(limits, fmt.Sprintf("runs-per-SQL=%d", runsPerStatement))
	}
	fmt.Printf(
		"Start random SQL workload: workers=%d, SQLs=%d, %s\n",
		workerCount,
		len(statements),
		strings.Join(limits, ", "),
	)

	workerCtx, stopWorkers := context.WithCancel(ctx)
	defer stopWorkers()
	schedulingCtx := workerCtx
	if runDuration > 0 {
		var stopTimer context.CancelFunc
		schedulingCtx, stopTimer = context.WithTimeout(workerCtx, runDuration)
		defer stopTimer()
	}

	scheduler := newRunSQLRandomScheduler(len(statements), runsPerStatement)
	succeededRuns := make([]atomic.Int64, len(statements))
	failedRuns := make([]atomic.Int64, len(statements))
	returnedRows := make([]atomic.Int64, len(statements))
	var totalFailures atomic.Int64
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
				statement := statements[statementIndex]
				label := fmt.Sprintf("SQL %d, run %d", statementIndex+1, runNumber)
				startedAt := time.Now()
				writeLog(
					"[worker %d] Start %s: %s",
					workerID,
					label,
					summarizeRunSQL(statement),
				)

				rowCount, err := execute(workerCtx, statementIndex, statement)
				elapsed := time.Since(startedAt)
				if err != nil {
					if workerCtx.Err() != nil {
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
	for i, statement := range statements {
		fmt.Printf(
			"Completed SQL %d: scheduled=%d, successful=%d, failed=%d, rows=%d, statement=%s\n",
			i+1,
			scheduledRuns[i],
			succeededRuns[i].Load(),
			failedRuns[i].Load(),
			returnedRows[i].Load(),
			summarizeRunSQL(statement),
		)
	}

	if ctx.Err() != nil {
		return fmt.Errorf("run-sqls canceled: %w", ctx.Err())
	}
	if failures := totalFailures.Load(); failures > 0 {
		return fmt.Errorf("run-sqls finished with %d failed execution(s)", failures)
	}
	fmt.Println("Success: random SQL workload finished")
	return nil
}

func executeRunSQL(ctx context.Context, db *sql.DB, setupStatements []string, statement string) (int64, error) {
	conn, err := db.Conn(ctx)
	if err != nil {
		return 0, fmt.Errorf("acquire database connection: %w", err)
	}
	defer conn.Close()

	for i, setupStatement := range setupStatements {
		if strings.TrimSpace(setupStatement) == "" {
			continue
		}
		if _, err := conn.ExecContext(ctx, setupStatement); err != nil {
			return 0, fmt.Errorf("execute setup SQL %d: %w", i+1, err)
		}
	}

	rows, err := conn.QueryContext(ctx, statement)
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
