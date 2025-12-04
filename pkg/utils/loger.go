package utils

import "log"

var debugMode bool

func SetDebug(enable bool) {
	debugMode = enable
}

func DebugLog(format string, v ...interface{}) {
	if debugMode {
		log.Printf("[DEBUG] "+format, v...)
	}
}
