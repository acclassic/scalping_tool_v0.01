package log

import (
	"fmt"
	"log"
	"os"
	"time"
)

func Sys_logger() *log.Logger {
	os.Mkdir("logs", 0750)
	logDate := time.Now().Format("2006-01-02")
	logPaht := fmt.Sprintf("logs/%s.log", logDate)
	logFile, _ := os.OpenFile(logPaht, os.O_APPEND|os.O_CREATE, 0644)
	logPrefix := time.Now().Format("2006-01-02 15:04:05 CEST")
	logger := log.New(logFile, logPrefix, log.Lshortfile)

	return logger
}

func Strat_logger() *log.Logger {
	os.Mkdir("analytics", 0750)
	logDate := time.Now().Format("2006-01-02")
	logPaht := fmt.Sprintf("logs/analytics/%s.log", logDate)
	logFile, _ := os.OpenFile(logPaht, os.O_APPEND|os.O_CREATE, 0644)
	logPrefix := time.Now().Format("2006-01-02 15:04:05 CEST")
	logger := log.New(logFile, logPrefix, log.Lshortfile)

	return logger
}
