package logging

import (
	"log"
	"runtime"
)

const (
	CONFIG   = "CONFIG"
	MACRO    = "MACRO"
	DAEMON   = "DAEMON"
	HANDLER  = "HANDLER"
	LOGIN    = "LOGIN"
	RENT     = "RENT"
	RERENT   = "RERENT"
	LOG      = "LOGGING"
	AUTH     = "AUTH"
	WATCHDOG = "WATCHDOG"
	MANAGER  = "MANAGER"
)

func PrintINFO(msg ...string) {
	_, file, line, ok := runtime.Caller(1)
	if !ok {
		file = "???"
		line = 0
	}
	log.Printf("[INFO] [%s:%d] %s", file, line, msg)
}

func PrintERROR(msg ...string) {
	_, file, line, ok := runtime.Caller(1)
	if !ok {
		file = "???"
		line = 0
	}
	log.Printf("[ERROR] [%s:%d] %s", file, line, msg)
}

func PrintWARNING(method string, msg ...string) {
	_, file, line, ok := runtime.Caller(1)
	if !ok {
		file = "???"
		line = 0
	}
	log.Printf("[WARN] [%s] [%s:%d] %s", method, file, line, msg)
}

func PrintDEBUG(method string, msg ...string) {
	_, file, line, ok := runtime.Caller(1)
	if !ok {
		file = "???"
		line = 0
	}
	log.Printf("[DEBUG] [%s] [%s:%d] %s", method, file, line, msg)
}
