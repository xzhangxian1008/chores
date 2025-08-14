package util

import (
	"log"
)

func DefaultMarkLog() {
	logImpl("--- mark ---")
}

func MarkLog(str string) {
	logImpl(str)
}

func logImpl(str string) {
	log.Println(str)
}
