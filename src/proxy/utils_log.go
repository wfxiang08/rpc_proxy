//// Copyright 2015 Spring Rain Software Compnay LTD. All Rights Reserved.
//// Licensed under the MIT (MIT-LICENSE.txt) license.
package proxy

import (
	log "github.com/wfxiang08/cyutils/utils/rolling_log"
	"os"
	"strings"
	"syscall"
	"time"
)

//
// 设置LogLevel
//
func SetLogLevel(level string) {
	var lv = log.LEVEL_INFO
	switch strings.ToLower(level) {
	case "error":
		lv = log.LEVEL_ERROR
	case "warn", "warning":
		lv = log.LEVEL_WARN
	case "debug":
		lv = log.LEVEL_DEBUG
	case "info":
		fallthrough
	default:
		lv = log.LEVEL_INFO
	}
	log.SetLevel(lv)
	log.Infof("set log level to %s", lv)
}

//
// 设置CrashLog
//
func SetCrashLog(file string) {
	f, err := os.OpenFile(file, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.InfoErrorf(err, "cannot open crash log file: %s", file)
	} else {
		syscall.Dup2(int(f.Fd()), 2)
	}
}

func FormatYYYYmmDDHHMMSS(date time.Time) string {
	return date.Format("2006-01-02 15:04:05")
}
