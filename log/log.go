package log

import (
	"log"

	"github.com/jonhadfield/gosn-v2/common"
)

func Fatal(msg string) {
	log.Fatal(common.LibName, "|", msg)
}

func Fatalf(format string, v ...interface{}) {
	log.Fatalf(format, v...)
}

func DebugPrint(show bool, msg string, maxChars int) {
	if show {
		if len(msg) > maxChars {
			msg = msg[:maxChars] + "..."
		}

		log.Println(common.LibName, "|", msg)
	}
}

func Println(v ...any) {
	log.Println(v...)
}
