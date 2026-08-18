package main

import (
	"database/sql"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestBuildInsertTasksDistributesRowsAndPassesConfig(t *testing.T) {
	db := &sql.DB{}
	config := insertConfig{
		totalRowCnt: 10,
		batchSize:   3,
		tableName:   "test_table",
		colInfos:    []colInfo{{tp: intType}},
		db:          db,
	}

	tasks := buildInsertTasks(config, 3)
	if len(tasks) != 3 {
		t.Fatalf("task count = %d, want 3", len(tasks))
	}

	gotCounts := make([]int, len(tasks))
	for i, task := range tasks {
		gotCounts[i] = task.insertCnt
		if task.taskID != i {
			t.Errorf("tasks[%d].taskID = %d, want %d", i, task.taskID, i)
		}
		if task.config.db != db || task.config.tableName != config.tableName || task.config.batchSize != config.batchSize {
			t.Errorf("tasks[%d] did not receive the initialized insert config", i)
		}
	}
	if want := []int{4, 3, 3}; !reflect.DeepEqual(gotCounts, want) {
		t.Fatalf("insert counts = %v, want %v", gotCounts, want)
	}
}

func TestCloneInsertColInfosClonesValues(t *testing.T) {
	original := []colInfo{{tp: intType, values: []string{"1", "2"}}}
	cloned := cloneInsertColInfos(original)
	cloned[0].values[0] = "changed"

	if original[0].values[0] != "1" {
		t.Fatalf("clone changed the original values slice: %v", original[0].values)
	}
}

func TestInitializeInsertColInfosUsesValuesGenerator(t *testing.T) {
	callCount := 0
	original := []colInfo{{
		tp:     intType,
		values: []string{"old"},
		valuesGenerator: func() []string {
			callCount++
			return []string{"generated-1", "generated-2"}
		},
	}}

	initialized := initializeInsertColInfos(original)
	if callCount != 1 {
		t.Fatalf("valuesGenerator call count = %d, want 1", callCount)
	}
	if want := []string{"generated-1", "generated-2"}; !reflect.DeepEqual(initialized[0].values, want) {
		t.Fatalf("initialized values = %v, want %v", initialized[0].values, want)
	}
	if want := []string{"old"}; !reflect.DeepEqual(original[0].values, want) {
		t.Fatalf("initialization changed original values = %v, want %v", original[0].values, want)
	}
}

func TestTruncateInsertFailedSQL(t *testing.T) {
	short := "insert into t values (1)"
	if got := truncateInsertFailedSQL(short); got != short {
		t.Fatalf("short SQL = %q, want %q", got, short)
	}

	got := truncateInsertFailedSQL(strings.Repeat("数", insertFailedSQLMaxRunes+1))
	if runeCount := utf8.RuneCountInString(got); runeCount != insertFailedSQLMaxRunes {
		t.Fatalf("truncated SQL has %d characters, want %d", runeCount, insertFailedSQLMaxRunes)
	}
	if !strings.HasSuffix(got, "...") {
		t.Fatalf("truncated SQL does not end with ...: %q", got)
	}
}
