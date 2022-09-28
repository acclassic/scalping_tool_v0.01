package log

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func get_path(path string) string {
	wDir, _ := os.Getwd()
	basePath := strings.SplitAfter(wDir, "scalping_tool_v0.01")
	filePath := filepath.Join(basePath[0], path)
	return filePath
}

//TODO change file name format and prefix format then save and git commit
func Sys_logger() *log.Logger {
	path := get_path("/logs")
	os.Mkdir(path, 0750)
	logDate := time.Now().Format("2006-01-02")
	logPaht := fmt.Sprintf("%s/%s.log", path, logDate)
	logFile, _ := os.OpenFile(logPaht, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0664)
	logPrefix := time.Now().Format("[02-01-2006 15:04:05 CEST] ")
	logger := log.New(logFile, logPrefix, log.Lshortfile)

	return logger
}

func Strat_logger() *log.Logger {
	path := get_path("/logs/analytics")
	os.Mkdir(path, 0750)
	logDate := time.Now().Format("2006-01-02")
	logPaht := fmt.Sprintf("%s/%s.log", path, logDate)
	logFile, _ := os.OpenFile(logPaht, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0664)
	logPrefix := time.Now().Format("[02-01-2006 15:04:05 CEST] ")
	logger := log.New(logFile, logPrefix, log.Lshortfile)

	return logger
}
