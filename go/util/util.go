package util

import (
	"log"
)

func MarkDefaultLog() {
	log.Println("--- mark ---")
}

func MarkLog(str string) {
	log.Println(str)
}
