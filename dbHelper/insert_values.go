package main

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ---------------------- INSERT CONFIG ----------------------
// All manually editable settings for the insert task are kept here.

var (
	insertDBConfig = dbConfig{
		address: defaultAddress,
		port:    defaultPort,
		user:    defaultUser,
		dbName:  defaultDBName,
		params:  make([]string, 0),
	}

	insertTotalRowCnt = 20000000
	insertBatchSize   = 1000
	insertTableName   = "ct2"
	insertTaskCount   = 20

	insertColInfos = []colInfo{
		{
			tp:              intType,
			nullProbability: -1,
		},
		{
			tp:              intType,
			nullProbability: 1,
			valuesGenerator: func() []string {
				cnt := 50000000
				s := make([]string, 0, cnt)
				for i := range cnt {
					s = append(s, strconv.Itoa(i))
				}
				return s
			},
		},
		{
			tp:              decimalType,
			nullProbability: 1,
			valuesGenerator: func() []string {
				cnt := 50000000
				s := make([]string, 0, cnt)
				for i := range cnt {
					s = append(s, fmt.Sprintf("%.2f", rand.Float32()+float32(i)))
				}
				return s
			},
		},
		{
			tp:              stringType,
			nullProbability: 1,
			valuesGenerator: func() []string {
				cnt := 50000000
				asciiChars := "abcdefghijklmnopqrstuvwxyz1234567890"
				s := make([]string, 0, cnt)
				for i := range cnt {
					var result []rune
					for range 30 {
						result = append(result, rune(asciiChars[rand.Intn(len(asciiChars))]))
					}

					s = append(s, fmt.Sprintf("%s-%d", string(result), i))
				}

				return s
			},
		},
		{
			tp:              datetimeType,
			nullProbability: 1,
			valuesGenerator: func() []string {
				cnt := 50000000
				s := make([]string, 0, cnt)
				for range cnt {
					s = append(s, fmt.Sprintf("%d-%d-%d %d:%d:%d", rand.Intn(500)+2000, rand.Intn(12)+1, rand.Intn(25)+1, rand.Intn(24), rand.Intn(60), rand.Intn(60)))
				}
				return s
			},
		},
	}
)

// ------------------------------------------------------------

const (
	intType = iota
	stringType
	decimalType
	floatType
	timeType
	dateType
	datetimeType
)

const (
	insertProgressInterval  = 50000
	insertFailedSQLMaxRunes = 500
)

type colInfo struct {
	tp int

	// This is the filed for nvd. When nvd > 0, we will generate values in advance and
	// store them in this slice.
	// If len(values) is 0, it means that nvd is equal to 0
	values []string

	valuesGenerator func() []string

	// -1 means that null is forbidden for this col
	// 60 means that we have 60% to generate null value for this column
	nullProbability int

	// This two fields is useful only when column type is string
	minStrLen int
	maxStrLen int
}

type insertConfig struct {
	totalRowCnt int
	batchSize   int
	tableName   string
	colInfos    []colInfo

	db  *sql.DB
	ctx context.Context
}

type insertTask struct {
	config    insertConfig
	taskID    int
	insertCnt int
}

func generateOneColValue(col colInfo) (retVal string) {
	if col.nullProbability > 0 && rand.Intn(100) < col.nullProbability {
		return "null"
	}

	defer func() {
		retVal = fmt.Sprintf("'%s'", retVal)
	}()

	if len(col.values) > 0 {
		retVal = col.values[rand.Intn(len(col.values))]
		return
	}

	switch col.tp {
	case intType:
		retVal = strconv.Itoa(int(rand.Int31()))
		return
	case stringType:
		retVal = generateRandomString(col.minStrLen, col.maxStrLen)
		return
	case decimalType, floatType:
		retVal = generateRandomFloat()
		return
	case timeType:
		retVal = generateRandomTime()
		return
	case dateType:
		retVal = generateRandomDate()
		return
	case datetimeType:
		retVal = generateRandomDatetime()
		return
	}

	panic(fmt.Sprintf("Unrecognized type %d", col.tp))
}

func generateOneRow(colInfos []colInfo) string {
	row := ""
	for i, col := range colInfos {
		val := generateOneColValue(col)
		if i == 0 {
			row = fmt.Sprintf("%s", val)
		} else {
			row = fmt.Sprintf("%s, %s", row, val)
		}
	}

	return fmt.Sprintf("(%s)", row)
}

func startToInsertForOneTask(task insertTask) error {
	db := task.config.db
	ctx := task.config.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	batchSize := task.config.batchSize
	tableName := task.config.tableName
	colInfos := task.config.colInfos

	if db == nil {
		return fmt.Errorf("insert task %d: database connection is nil", task.taskID)
	}

	nextProgress := insertProgressInterval
	for inserted := 0; inserted < task.insertCnt; {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("insert task %d canceled: %w", task.taskID, err)
		}

		rowCount := min(batchSize, task.insertCnt-inserted)
		rows := make([]string, 0, rowCount)
		for range rowCount {
			rows = append(rows, generateOneRow(colInfos))
		}

		insertSQL := fmt.Sprintf("insert into %s values %s", tableName, strings.Join(rows, ", "))
		if _, err := db.ExecContext(ctx, insertSQL); err != nil {
			if ctx.Err() != nil {
				return fmt.Errorf("insert task %d canceled: %w", task.taskID, ctx.Err())
			}
			fmt.Printf("[insert task %d] Insert failed: %v\n", task.taskID, err)
			fmt.Printf("[insert task %d] Failed SQL: %s\n", task.taskID, truncateInsertFailedSQL(insertSQL))
			return fmt.Errorf("insert task %d: execute insert: %w", task.taskID, err)
		}
		inserted += rowCount
		for inserted >= nextProgress {
			fmt.Printf("[insert task %d] Inserted %d rows\n", task.taskID, nextProgress)
			nextProgress += insertProgressInterval
		}
	}

	return nil
}

func truncateInsertFailedSQL(statement string) string {
	return truncateRunes(statement, insertFailedSQLMaxRunes)
}

func validateInsertSettings() error {
	if insertTotalRowCnt <= 0 {
		return fmt.Errorf("insertTotalRowCnt must be greater than 0, got %d", insertTotalRowCnt)
	}
	if insertBatchSize <= 0 {
		return fmt.Errorf("insertBatchSize must be greater than 0, got %d", insertBatchSize)
	}
	if strings.TrimSpace(insertTableName) == "" {
		return fmt.Errorf("insertTableName must not be empty")
	}
	if insertTaskCount <= 0 {
		return fmt.Errorf("insertTaskCount must be greater than 0, got %d", insertTaskCount)
	}
	if len(insertColInfos) == 0 {
		return fmt.Errorf("insertColInfos must contain at least one column")
	}

	for i, col := range insertColInfos {
		if col.tp < intType || col.tp > datetimeType {
			return fmt.Errorf("insertColInfos[%d] has an unrecognized type %d", i, col.tp)
		}
		if col.nullProbability < -1 || col.nullProbability > 100 {
			return fmt.Errorf("insertColInfos[%d].nullProbability must be between -1 and 100", i)
		}
		if col.tp == stringType && len(col.values) == 0 {
			if col.minStrLen < 0 || col.maxStrLen < col.minStrLen {
				return fmt.Errorf("insertColInfos[%d] has an invalid string length range [%d, %d]", i, col.minStrLen, col.maxStrLen)
			}
		}
	}

	return nil
}

func cloneInsertColInfos(colInfos []colInfo) []colInfo {
	cloned := make([]colInfo, len(colInfos))
	copy(cloned, colInfos)
	for i := range cloned {
		cloned[i].values = append([]string(nil), colInfos[i].values...)
	}
	return cloned
}

func initializeInsertColInfos(colInfos []colInfo) []colInfo {
	initialized := cloneInsertColInfos(colInfos)
	for i := range initialized {
		if initialized[i].valuesGenerator != nil {
			initialized[i].values = append([]string(nil), initialized[i].valuesGenerator()...)
		}
	}
	return initialized
}

func initializeInsertConfig() (insertConfig, error) {
	if err := validateInsertSettings(); err != nil {
		return insertConfig{}, fmt.Errorf("invalid insert config: %w", err)
	}

	colInfos := initializeInsertColInfos(insertColInfos)
	db, err := getDB(insertDBConfig)
	if err != nil {
		return insertConfig{}, fmt.Errorf("initialize insert database: %w", err)
	}

	return insertConfig{
		totalRowCnt: insertTotalRowCnt,
		batchSize:   insertBatchSize,
		tableName:   insertTableName,
		colInfos:    colInfos,
		db:          db,
	}, nil
}

func printInsertConfig(config insertConfig) {
	fmt.Println("========== INSERT CONFIG ==========")
	fmt.Printf("insertDBConfig.address=%q\n", insertDBConfig.address)
	fmt.Printf("insertDBConfig.port=%q\n", insertDBConfig.port)
	fmt.Printf("insertDBConfig.user=%q\n", insertDBConfig.user)
	fmt.Printf("insertDBConfig.dbName=%q\n", insertDBConfig.dbName)
	fmt.Printf("insertDBConfig.params=%q\n", insertDBConfig.params)
	fmt.Printf("insertTotalRowCnt=%d\n", config.totalRowCnt)
	fmt.Printf("insertBatchSize=%d\n", config.batchSize)
	fmt.Printf("insertTableName=%q\n", config.tableName)
	fmt.Printf("insertTaskCount=%d\n", insertTaskCount)
	fmt.Printf("insertColInfos.count=%d\n", len(config.colInfos))
	for i, col := range config.colInfos {
		fmt.Printf(
			"insertColInfos[%d]: type=%d, nullProbability=%d, minStrLen=%d, maxStrLen=%d, valuesCount=%d, hasValuesGenerator=%t\n",
			i,
			col.tp,
			col.nullProbability,
			col.minStrLen,
			col.maxStrLen,
			len(col.values),
			col.valuesGenerator != nil,
		)
	}
	fmt.Println("========== END INSERT CONFIG ==========")
}

func buildInsertTasks(config insertConfig, taskCount int) []insertTask {
	tasks := make([]insertTask, 0, taskCount)
	rowsPerTask := config.totalRowCnt / taskCount
	extraRows := config.totalRowCnt % taskCount

	for taskID := range taskCount {
		insertCnt := rowsPerTask
		if taskID < extraRows {
			insertCnt++
		}
		tasks = append(tasks, insertTask{
			config:    config,
			taskID:    taskID,
			insertCnt: insertCnt,
		})
	}

	return tasks
}

func runInsertValues() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	config, err := initializeInsertConfig()
	if err != nil {
		return err
	}
	config.ctx = ctx
	printInsertConfig(config)

	insertTasks := buildInsertTasks(config, insertTaskCount)
	runnerTasks := make([]func(), 0, len(insertTasks))

	startTime := time.Now()
	fmt.Printf("[insert] Started at %s\n", startTime.Format(time.RFC3339Nano))
	defer func() {
		endTime := time.Now()
		fmt.Printf(
			"[insert] Finished at %s, elapsed=%s\n",
			endTime.Format(time.RFC3339Nano),
			endTime.Sub(startTime),
		)
	}()

	var runErr error
	var runErrMu sync.Mutex
	for _, task := range insertTasks {
		runnerTasks = append(runnerTasks, func() {
			runErrMu.Lock()
			if runErr != nil {
				runErrMu.Unlock()
				return
			}
			runErrMu.Unlock()

			err := startToInsertForOneTask(task)
			if err == nil {
				return
			}

			runErrMu.Lock()
			if runErr == nil {
				runErr = err
				cancel()
			}
			runErrMu.Unlock()
		})
	}

	newRunner(runnerTasks).run()
	closeErr := config.db.Close()
	runErrMu.Lock()
	firstRunErr := runErr
	runErrMu.Unlock()
	if firstRunErr != nil {
		return firstRunErr
	}
	if closeErr != nil {
		return fmt.Errorf("close insert database: %w", closeErr)
	}
	return nil
}
