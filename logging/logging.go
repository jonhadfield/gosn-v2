package logging

import (
	"github.com/jonhadfield/gosn-v2/common"
	"log"
)

func DebugPrint(show bool, msg string, maxChars int) {
	if show {
		if len(msg) > maxChars {
			msg = msg[:maxChars] + "..."
		}

		log.Println(common.LibName, "|", msg)
	}
}
