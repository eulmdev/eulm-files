package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

func colourCode(r, g, b uint8) string {
	return fmt.Sprintf("\x1b[38;2;%d;%d;%dm", r, g, b)
}

type Logger struct {
	timezone *time.Location

	successCol string
	infoCol    string
	warnCol    string
	errorCol   string
	fatalCol   string

	successDimCol string
	infoDimCol    string
	warnDimCol    string
	errorDimCol   string
	fatalDimCol   string

	resetCode string
}

func (l *Logger) Info(args ...string) {
	var str string

	for _, val := range args {
		str += val + " "
	}

	t := time.Now().In(l.timezone).Format("02/01/2006 15:04:05")
	fmt.Printf("%sINF %s - %s%s%s\n", l.infoDimCol, t, l.infoCol, str, l.resetCode)
}

func (l *Logger) Warn(args ...string) {
	var str string

	for _, val := range args {
		str += val + " "
	}

	t := time.Now().In(l.timezone).Format("02/01/2006 15:04:05")
	fmt.Printf("%sWRN %s - %s%s%s\n", l.warnDimCol, t, l.warnCol, str, l.resetCode)
}

func (l *Logger) Error(args ...string) {
	var str string

	for _, val := range args {
		str += val + " "
	}

	t := time.Now().In(l.timezone).Format("02/01/2006 15:04:05")
	fmt.Printf("%sERR %s - %s%s%s\n", l.errorDimCol, t, l.errorCol, str, l.resetCode)
}

func (l *Logger) Fatal(args ...string) {
	var str string

	for _, val := range args {
		str += val + " "
	}

	t := time.Now().In(l.timezone).Format("02/01/2006 15:04:05")
	fmt.Printf("%sERR %s - %s%s%s\n", l.fatalDimCol, t, l.fatalCol, str, l.resetCode)

	os.Exit(1)
}

func newLogger() *Logger {
	location, err := time.LoadLocation("Europe/London")
	if err != nil {
		location = time.Local
	}

	return &Logger{
		timezone: location,

		infoCol:  colourCode(233, 215, 90),
		warnCol:  colourCode(255, 135, 47),
		errorCol: colourCode(255, 70, 50),
		fatalCol: colourCode(181, 29, 29),

		infoDimCol:  colourCode(202, 191, 111),
		warnDimCol:  colourCode(216, 140, 84),
		errorDimCol: colourCode(216, 98, 86),
		fatalDimCol: colourCode(169, 72, 72),

		resetCode: "\x1b[0m",
	}
}

var logger = newLogger()

func respondJSON(w http.ResponseWriter, status int, payload map[string]any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		logger.Error("Error writing JSON to response:", err.Error())
	}
}

func respondFile(w http.ResponseWriter, status int, fileName string, fileData []byte) {
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", fileName))
	w.Header().Set("Content-Type", "application/octet-stream")
	w.WriteHeader(status)
	if _, err := w.Write(fileData); err != nil {
		logger.Error("Error writing file data to response:", err.Error())
	}
}
