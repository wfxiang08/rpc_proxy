//// Copyright 2015 Spring Rain Software Compnay LTD. All Rights Reserved.
//// Licensed under the MIT (MIT-LICENSE.txt) license.
package proxy

import (
	"fmt"
	"github.com/docopt/docopt-go"
	log "github.com/wfxiang08/cyutils/utils/rolling_log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"strconv"
)

var usage = `Usage: 
  %s -c <config_file> [-L <log_file>] [--log-level=<loglevel>] [--log-keep-days=<maxdays>] [--profile-addr=<profile-addr>] [--work-dir=<work-dir>] [--code-url-version=<code-url-version>]
  %s -V | --version

options:
   -c <config_file>
   -L	set output log file, default is stdout
   --log-level=<loglevel>	set log level: info, warn, error, debug [default: info]
   --log-keep-days=<maxdays>  set max log file keep days, default is 3 days
   --profile-addr=<profile-addr>
   --work-dir=<work-dir>
   --code-url-version=<code-url-version>
`

func RpcMain(binaryName string, serviceDesc string, configCheck ConfigCheck,
	serverFactory ServerFactorory, buildDate string, gitVersion string) {

	// 1. 准备解析参数
	usage = fmt.Sprintf(usage, binaryName, binaryName)

	version := fmt.Sprintf("Version: %s\nBuildDate: %s\nDesc: %s\nAuthor: wangfei@chunyu.me", gitVersion, buildDate, serviceDesc)
	args, err := docopt.Parse(usage, nil, true, version, true)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if s, ok := args["-V"].(bool); ok && s {
		fmt.Println(Green(version))
		os.Exit(1)
	}

	// 这就是为什么 Codis 傻乎乎起一个 http server的目的
	if s, ok := args["--profile-addr"].(string); ok && len(s) > 0 {
		go func() {
			log.Printf(Red("Profile Address: %s"), s)
			log.Println(http.ListenAndServe(s, nil))
		}()
	}

	// 2. 解析Log相关的配置
	log.SetLevel(log.LEVEL_INFO)

	var maxKeepDays int = 3
	if s, ok := args["--log-keep-days"].(string); ok && s != "" {
		v, err := strconv.ParseInt(s, 10, 32)
		if err != nil {
			log.PanicErrorf(err, "invalid max log file keep days = %s", s)
		}
		maxKeepDays = int(v)
	}

	// set output log file
	if s, ok := args["-L"].(string); ok && s != "" {
		f, err := log.NewRollingFile(s, maxKeepDays)
		if err != nil {
			log.PanicErrorf(err, "open rolling log file failed: %s", s)
		} else {
			defer f.Close()
			log.StdLog = log.New(f, "")
		}
	}
	log.SetLevel(log.LEVEL_INFO)
	log.SetFlags(log.Flags() | log.Lshortfile)

	// set log level
	if s, ok := args["--log-level"].(string); ok && s != "" {
		SetLogLevel(s)
	}

	// 没有就没有
	workDir, _ := args["--work-dir"].(string)
	codeUrlVersion, _ := args["--code-url-version"].(string)
	if len(workDir) == 0 {
		workDir, _ = os.Getwd()
	}

	log.Printf("WorkDir: %s, CodeUrl: %s", workDir, codeUrlVersion)

	// 3. 解析Config
	configFile := args["-c"].(string)
	conf, err := LoadConf(configFile)
	if err != nil {
		log.PanicErrorf(err, "load config failed")
	}

	// 额外的配置信息
	conf.WorkDir = workDir
	conf.CodeUrlVersion = codeUrlVersion

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
