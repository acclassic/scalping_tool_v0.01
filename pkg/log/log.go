package log

import (
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var muAnlytFile sync.Mutex

func get_path(path string) string {
	wDir, _ := os.Getwd()
	basePath := strings.SplitAfter(wDir, "scalping_tool_v0.01")
	filePath := filepath.Join(basePath[0], path)
	return filePath
}

func Init_folder_struct() {
	//Create log folder
	os.Mkdir(get_path("logs"), 0750)
	//Create analytics log folder
	os.Mkdir(get_path("logs/analytics"), 0750)
	//Create analytics folder
	os.Mkdir(get_path("analytics"), 0750)
}

func Sys_logger() *log.Logger {
	path := get_path("/logs")
	logDate := time.Now().Format("2006-01-02")
	logPaht := fmt.Sprintf("%s/%s.log", path, logDate)
	logFile, _ := os.OpenFile(logPaht, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0664)
	logPrefix := time.Now().Format("[02-01-2006 15:04:05 CEST] ")
	logger := log.New(logFile, logPrefix, log.Lshortfile)

	return logger
}

func Strat_logger() *log.Logger {
	path := get_path("/logs/analytics")
	logDate := time.Now().Format("2006-01-02")
	logPaht := fmt.Sprintf("%s/%s.log", path, logDate)
	logFile, _ := os.OpenFile(logPaht, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0664)
	logPrefix := time.Now().Format("[02-01-2006 15:04:05 CEST] ")
	logger := log.New(logFile, logPrefix, log.Lshortfile)

	return logger
}

func Add_analytics(id int, market string, price, qty float64) {
	muAnlytFile.Lock()
	path := get_path("/analytics")
	date := time.Now().Format("2006-01-02")
	filePath := fmt.Sprintf("%s/analytics_%s.csv", path, date)
	file, _ := os.OpenFile(filePath, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0664)
	defer file.Close()
	csvWriter := csv.NewWriter(file)
	csvWriter.Write([]string{fmt.Sprint(id), market, fmt.Sprint(price), fmt.Sprint(qty)})
	csvWriter.Flush()
	muAnlytFile.Unlock()
}
