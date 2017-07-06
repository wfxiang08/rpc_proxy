//// Copyright 2015 Spring Rain Software Compnay LTD. All Rights Reserved.
//// Licensed under the MIT (MIT-LICENSE.txt) license.
package proxy

import (
	"flag"
	"fmt"
	log "github.com/wfxiang08/cyutils/utils/rolling_log"
	"net/http"
	_ "net/http/pprof"
	"os"
)

//-flag
//-flag=x
//-flag x
var (
	configFile     = flag.String("c", "", "config file")
	version        = flag.Bool("V", false, "code-url-version")
	logFile        = flag.String("L", "", "set output log file, default is stdout")
	logLevel       = flag.String("log_level", "", "work-dir")
	profileAddr    = flag.String("profile_address", "", "profile address")
	workDirFlag    = flag.String("work_dir", "", "work-dir")
	codeUrlVersion = flag.String("code_url_version", "", "code-url-version")
)

func RpcMain(binaryName string, serviceDesc string, configCheck ConfigCheck,
	serverFactory ServerFactorory, buildDate string, gitVersion string) {

	flag.Parse()

	// 1. 准备解析参数
	if *version {
		versionStr := fmt.Sprintf("Version: %s\nBuildDate: %s\nDesc: %s\nAuthor: wfxiang08@gmail.com", gitVersion, buildDate, serviceDesc)
		fmt.Println(Green(versionStr))
		os.Exit(1)
	}

	// 这就是为什么 Codis 傻乎乎起一个 http server的目的
	if len(*profileAddr) > 0 {
		go func() {
			log.Printf(Red("Profile Address: %s"), *profileAddr)
			log.Println(http.ListenAndServe(*profileAddr, nil))
		}()
	}

	// 2. 解析Log相关的配置
	log.SetLevel(log.LEVEL_INFO)

	var maxKeepDays int = 3

	// set output log file
	if len(*logFile) > 0 {
		f, err := log.NewRollingFile(*logFile, maxKeepDays)

		if err != nil {
			log.PanicErrorf(err, "open rolling log file failed: %s", *logFile)
		} else {
			defer f.Close()
			log.StdLog = log.New(f, "")
		}
	}
	log.SetLevel(log.LEVEL_INFO)
	log.SetFlags(log.Flags() | log.Lshortfile)

	// set log level
	if len(*logLevel) > 0 {
		SetLogLevel(*logLevel)
	}

	// 没有就没有
	var workDir string
	if len(*workDirFlag) == 0 {
		workDir, _ = os.Getwd()
	} else {
		workDir = *workDirFlag
	}

	log.Printf("WorkDir: %s, CodeUrl: %s", workDir, codeUrlVersion)

	// 3. 解析Config
	if len(*configFile) == 0 {
		log.Panicf("Config file not specified")
	}
	conf, err := LoadConf(*configFile)
	if err != nil {
		log.PanicErrorf(err, "load config failed")
	}

	// 额外的配置信息
	conf.WorkDir = workDir
	conf.CodeUrlVersion = *codeUrlVersion

	if configCheck != nil {
		configCheck(conf)
	} else {
		log.Panic("No Config Check Given")
	}
	// 每次启动的时候都打印版本信息
	log.Infof(Green("-----------------\n%s\n--------------------------------------------------------------------"), version)

	// 启动服务
	server := serverFactory(conf)
	server.Run()
}
