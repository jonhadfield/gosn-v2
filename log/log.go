package log

import (
	"github.com/jonhadfield/gosn-v2/common"
	"log"
)

func Fatal(msg string) {
	log.Fatal(common.LibName, "|", msg)
}

func DebugPrint(show bool, msg string, maxChars int) {
	if show {
		if len(msg) > maxChars {
			msg = msg[:maxChars] + "..."
		}

		log.Println(common.LibName, "|", msg)
	}
}
