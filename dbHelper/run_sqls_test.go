package main

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"
)

func TestRunSQLWorkloadHonorsPerStatementLimitAndPrintsRows(t *testing.T) {
	setRunSQLConfigForTest(t, 5, 0, 10)
	statements := []string{
		"select 1",
		"select 1 union all select 2",
		"select 1 union all select 2 union all select 3",
	}
	completed := make([]atomic.Int64, len(statements))
	var runErr error
	output := captureStdout(t, func() {
		runErr = runSQLWorkload(
			context.Background(),
			statements,
			func(_ context.Context, statementIndex int, _ string) (int64, error) {
				completed[statementIndex].Add(1)
				return int64(statementIndex + 1), nil
			},
		)
	})
	if runErr != nil {
		t.Fatalf("SQL workload failed: %v", runErr)
	}
	for i := range statements {
		if got := completed[i].Load(); got != 10 {
			t.Fatalf("SQL %d completed %d runs, want 10", i+1, got)
		}
		wantSummary := fmt.Sprintf(
			"Completed SQL %d: scheduled=10, successful=10, failed=0, rows=%d",
			i+1,
			(i+1)*10,
		)
		if !strings.Contains(output, wantSummary) {
			t.Fatalf("output does not contain %q:\n%s", wantSummary, output)
		}
	}
	if !strings.Contains(output, "Success SQL") || !strings.Contains(output, "rows=") {
		t.Fatalf("per-run row-count log is missing:\n%s", output)
	}
}

func TestRunSQLWorkloadContinuesAfterErrorsAndTruncatesThem(t *testing.T) {
	setRunSQLConfigForTest(t, 1, 0, 3)
	longError := errors.New(strings.Repeat("错误", 80))
	var attempts atomic.Int64
	var runErr error
	output := captureStdout(t, func() {
		runErr = runSQLWorkload(
			context.Background(),
			[]string{"select broken"},
			func(context.Context, int, string) (int64, error) {
				attempts.Add(1)
				return 7, longError
			},
		)
	})
	if runErr == nil || !strings.Contains(runErr.Error(), "3 failed execution(s)") {
		t.Fatalf("unexpected workload error: %v", runErr)
	}
	if got := attempts.Load(); got != 3 {
		t.Fatalf("attempt count = %d, want 3; workload stopped too early", got)
	}

	failureLines := 0
	for _, line := range strings.Split(output, "\n") {
		markerIndex := strings.Index(line, "error=")
		if markerIndex < 0 || !strings.Contains(line, "Failed SQL") {
			continue
		}
		failureLines++
		errorText := line[markerIndex+len("error="):]
		if got := utf8.RuneCountInString(errorText); got > runSQLErrorMaxRunes {
			t.Fatalf("error log has %d characters, want at most %d: %q", got, runSQLErrorMaxRunes, errorText)
		}
		if !strings.HasSuffix(errorText, "...") {
			t.Fatalf("truncated error does not end with ...: %q", errorText)
		}
	}
	if failureLines != 3 {
		t.Fatalf("failure log count = %d, want 3:\n%s", failureLines, output)
	}
}

func TestRunSQLWorkloadHonorsDurationLimit(t *testing.T) {
	const workerCount = 3
	setRunSQLConfigForTest(t, workerCount, 80*time.Millisecond, 0)
	var attempts atomic.Int64
	startTime := time.Now()
	err := runSQLWorkload(
		context.Background(),
		[]string{"select sleep"},
		func(ctx context.Context, _ int, _ string) (int64, error) {
			attempts.Add(1)
			select {
			case <-time.After(10 * time.Millisecond):
				return 1, nil
			case <-ctx.Done():
				return 0, ctx.Err()
			}
		},
	)
	if err != nil {
		t.Fatalf("duration-limited workload failed: %v", err)
	}
	if got := attempts.Load(); got < workerCount {
		t.Fatalf("attempt count = %d, want at least one run per worker", got)
	}
	if elapsed := time.Since(startTime); elapsed < 80*time.Millisecond || elapsed > time.Second {
		t.Fatalf("duration-limited workload ran for %s, want between 80ms and 1s", elapsed)
	}
}

func TestExecuteRunSQLUsesOneSessionAndCountsRows(t *testing.T) {
	state := &runSQLTestDriverState{}
	driverName := fmt.Sprintf("run-sql-test-%d", time.Now().UnixNano())
	sql.Register(driverName, &runSQLTestDriver{state: state})
	db, err := sql.Open(driverName, "")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	rowCount, err := executeRunSQL(
		context.Background(),
		db,
		[]string{"use test", "set variable = 1"},
		"select value from t",
	)
	if err != nil {
		t.Fatalf("executeRunSQL failed: %v", err)
	}
	if rowCount != 3 {
		t.Fatalf("returned row count = %d, want 3", rowCount)
	}

	state.mu.Lock()
	defer state.mu.Unlock()
	if !reflect.DeepEqual(state.setupStatements, []string{"use test", "set variable = 1"}) {
		t.Fatalf("setup statements = %v", state.setupStatements)
	}
	if state.queryStatement != "select value from t" {
		t.Fatalf("query statement = %q", state.queryStatement)
	}
	if len(state.connectionIDs) != 3 {
		t.Fatalf("connection ID count = %d, want 3", len(state.connectionIDs))
	}
	for _, connectionID := range state.connectionIDs {
		if connectionID != state.connectionIDs[0] {
			t.Fatalf("setup and query used different connections: %v", state.connectionIDs)
		}
	}
}

func TestTruncateRunSQLErrorUsesCharacterLimit(t *testing.T) {
	short := errors.New("short error")
	if got := truncateRunSQLError(short); got != short.Error() {
		t.Fatalf("short error = %q, want %q", got, short.Error())
	}

	got := truncateRunSQLError(errors.New(strings.Repeat("错", 101)))
	if runeCount := utf8.RuneCountInString(got); runeCount != runSQLErrorMaxRunes {
		t.Fatalf("truncated error has %d characters, want %d", runeCount, runSQLErrorMaxRunes)
	}
	if !strings.HasSuffix(got, "...") {
		t.Fatalf("truncated error does not end with ...: %q", got)
	}
}

func TestValidateRunSQLConfig(t *testing.T) {
	setRunSQLConfigForTest(t, 1, 0, 0)
	if err := validateRunSQLConfig([]string{"select 1"}); err == nil || !strings.Contains(err.Error(), "requires a duration") {
		t.Fatalf("unexpected missing-limit error: %v", err)
	}

	runSQLRunsPerStatement = 1
	if err := validateRunSQLConfig(nil); err == nil || !strings.Contains(err.Error(), "at least one") {
		t.Fatalf("unexpected empty-list error: %v", err)
	}
	if err := validateRunSQLConfig([]string{"select 1", "   "}); err == nil || !strings.Contains(err.Error(), "[1] cannot be empty") {
		t.Fatalf("unexpected empty-SQL error: %v", err)
	}
}

func TestParseRunSQLsTaskName(t *testing.T) {
	for _, args := range [][]string{{"--task", "run-sqls"}, {"run_sqls"}, {"run-sql"}} {
		got, err := parseTaskName(args)
		if err != nil {
			t.Fatalf("parseTaskName(%v) failed: %v", args, err)
		}
		if got != runSQLsTaskName {
			t.Fatalf("parseTaskName(%v) = %q, want %q", args, got, runSQLsTaskName)
		}
	}
}

func setRunSQLConfigForTest(t *testing.T, workers int, duration time.Duration, runsPerStatement int) {
	t.Helper()
	originalWorkers := runSQLConcurrentWorkerCount
	originalDuration := runSQLRunDuration
	originalRunsPerStatement := runSQLRunsPerStatement
	runSQLConcurrentWorkerCount = workers
	runSQLRunDuration = duration
	runSQLRunsPerStatement = runsPerStatement
	t.Cleanup(func() {
		runSQLConcurrentWorkerCount = originalWorkers
		runSQLRunDuration = originalDuration
		runSQLRunsPerStatement = originalRunsPerStatement
	})
}

type runSQLTestDriverState struct {
	mu              sync.Mutex
	nextConnection  int
	setupStatements []string
	queryStatement  string
	connectionIDs   []int
}

type runSQLTestDriver struct {
	state *runSQLTestDriverState
}

func (d *runSQLTestDriver) Open(string) (driver.Conn, error) {
	d.state.mu.Lock()
	d.state.nextConnection++
	connectionID := d.state.nextConnection
	d.state.mu.Unlock()
	return &runSQLTestConnection{id: connectionID, state: d.state}, nil
}

type runSQLTestConnection struct {
	id    int
	state *runSQLTestDriverState
}

func (c *runSQLTestConnection) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("Prepare is not implemented")
}

func (c *runSQLTestConnection) Close() error { return nil }

func (c *runSQLTestConnection) Begin() (driver.Tx, error) {
	return nil, errors.New("Begin is not implemented")
}

func (c *runSQLTestConnection) ExecContext(_ context.Context, statement string, _ []driver.NamedValue) (driver.Result, error) {
	c.state.mu.Lock()
	defer c.state.mu.Unlock()
	c.state.setupStatements = append(c.state.setupStatements, statement)
	c.state.connectionIDs = append(c.state.connectionIDs, c.id)
	return driver.RowsAffected(0), nil
}

func (c *runSQLTestConnection) QueryContext(_ context.Context, statement string, _ []driver.NamedValue) (driver.Rows, error) {
	c.state.mu.Lock()
	c.state.queryStatement = statement
	c.state.connectionIDs = append(c.state.connectionIDs, c.id)
	c.state.mu.Unlock()
	return &runSQLTestRows{rowCount: 3}, nil
}

type runSQLTestRows struct {
	rowCount int
	current  int
}

func (r *runSQLTestRows) Columns() []string { return []string{"value"} }

func (r *runSQLTestRows) Close() error { return nil }

func (r *runSQLTestRows) Next(values []driver.Value) error {
	if r.current >= r.rowCount {
		return io.EOF
	}
	values[0] = int64(r.current)
	r.current++
	return nil
}
