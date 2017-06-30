//// Copyright 2015 Spring Rain Software Compnay LTD. All Rights Reserved.
//// Licensed under the MIT (MIT-LICENSE.txt) license.

package proxy

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/wfxiang08/cyutils/utils"
	"github.com/wfxiang08/cyutils/utils/atomic2"
	log "github.com/wfxiang08/cyutils/utils/rolling_log"
)

//
// 单个的统计指标
//
type OpStats struct {
	opstr string
	calls atomic2.Int64 // 次数
	usecs atomic2.Int64 // 总时间(us)
}

func (s *OpStats) OpStr() string {
	return s.opstr
}

func (s *OpStats) Calls() int64 {
	return s.calls.Get()
}

func (s *OpStats) USecs() int64 {
	return s.usecs.Get()
}

func (s *OpStats) USecsPerCall() int64 {
	var perusecs int64 = 0
	if s.calls.Get() != 0 {
		perusecs = s.usecs.Get() / s.calls.Get()
	}
	return perusecs
}

func (s *OpStats) MarshalJSON() ([]byte, error) {
	var m = make(map[string]interface{})
	var calls = s.calls.Get()
	var usecs = s.usecs.Get()

	var perusecs int64 = 0
	if calls != 0 {
		perusecs = usecs / calls
	}

	m["cmd"] = s.opstr
	m["calls"] = calls
	m["usecs"] = usecs
	m["usecs_percall"] = perusecs
	return json.Marshal(m)
}

//
// 所有的Commands的统计信息
//
var cmdstats struct {
	requests atomic2.Int64

	opmap map[string]*OpStats

	histMaps chan *OpStatsInfo
	rwlck    sync.RWMutex
	ticker   *time.Ticker
	start    sync.Once
}

type OpStatsInfo struct {
	opmap     map[string]*OpStats
	timestamp time.Time
}

func init() {
	cmdstats.opmap = make(map[string]*OpStats)
}

const (
	EMPTY_STR = ""
)

// 主动调用
func StartTicker(falconClient string, service string) {
	// 如果没有监控配置，则直接返回
	if len(falconClient) == 0 {
		return
	}

	log.Printf(Green("Log to falconClient: %s"), falconClient)

	cmdstats.histMaps = make(chan *OpStatsInfo, 5)
	cmdstats.ticker = time.NewTicker(time.Minute)

	var statsInfo *OpStatsInfo
	hostname := utils.Hostname()
	go func() {
		for true {
			statsInfo = <-cmdstats.histMaps

			// 准备发送
			// 需要处理timeout
			metrics := make([]*utils.MetaData, 0, 3)
			t := statsInfo.timestamp.Unix()

			for method, stats := range statsInfo.opmap {

				metricCount := &utils.MetaData{
					Metric:      fmt.Sprintf("%s.%s.calls", service, method),
					Endpoint:    hostname,
					Value:       stats.Calls(),
					CounterType: utils.DATA_TYPE_GAUGE,
					Tags:        EMPTY_STR,
					Timestamp:   t,
					Step:        60, // 一分钟一次采样
				}

				metricAvg := &utils.MetaData{
					Metric:      fmt.Sprintf("%s.%s.avgrt", service, method),
					Endpoint:    hostname,
					Value:       float64(stats.USecsPerCall()) * 0.001, // 单位: ms
					CounterType: utils.DATA_TYPE_GAUGE,
					Tags:        EMPTY_STR,
					Timestamp:   t,
					Step:        60, // 一分钟一次采样
				}

				metrics = append(metrics, metricCount, metricAvg)
			}

			// 准备发送数据到Local Agent
			// 10s timeout
			log.Printf("Send %d Metrics....", len(metrics))
			if len(metrics) > 0 {
				utils.SendData(metrics, falconClient, time.Second*10)
			}

		}

	}()

	go func() {
		// 死循环: 最终进程退出时自动被杀掉
		var t time.Time
		for t = range cmdstats.ticker.C {
			// 到了指定的时间点之后将过去一分钟的统计数据转移到:
			cmdstats.rwlck.Lock()
			cmdstats.histMaps <- &OpStatsInfo{
				opmap:     cmdstats.opmap,
				timestamp: t,
			}
			cmdstats.opmap = make(map[string]*OpStats)
			cmdstats.rwlck.Unlock()
		}
	}()
}

func OpCounts() int64 {
	return cmdstats.requests.Get()
}

func GetOpStats(methodName string, create bool) *OpStats {
	cmdstats.rwlck.RLock()
	s := cmdstats.opmap[methodName]
	cmdstats.rwlck.RUnlock()

	if s != nil || !create {
		return s
	}

	cmdstats.rwlck.Lock()
	s = cmdstats.opmap[methodName]
	if s == nil {
		s = &OpStats{opstr: methodName}
		cmdstats.opmap[methodName] = s
	}
	cmdstats.rwlck.Unlock()
	return s
}

func GetAllOpStats() []*OpStats {
	var all = make([]*OpStats, 0, 128)
	cmdstats.rwlck.RLock()
	for _, s := range cmdstats.opmap {
		all = append(all, s)
	}
	cmdstats.rwlck.RUnlock()
	return all
}

func incrOpStats(methodName string, usecs int64) {
	s := GetOpStats(methodName, true)
	s.calls.Incr()
	s.usecs.Add(usecs)
	cmdstats.requests.Incr()
}
