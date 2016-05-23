// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"google.golang.org/api/monitoring/v3"

	"v.io/v23/verror"
	"v.io/x/lib/gcm"
)

const (
	defaultDuration = time.Hour
	gcmAuthTimeout  = time.Hour
)

var (
	dashboardIndexCache = []byte{}
	gcmService          *monitoring.Service
	gcmLastAuthTime     time.Time
)

type point struct {
	Timestamp int64
	Value     float64
}
type points []point

func (pts points) Len() int           { return len(pts) }
func (pts points) Less(i, j int) bool { return pts[i].Timestamp < pts[j].Timestamp }
func (pts points) Swap(i, j int)      { pts[i], pts[j] = pts[j], pts[i] }
func (pts points) Sort()              { sort.Sort(pts) }

type statsResult struct {
	SysMemUsageBytes  points
	SysMemUsagePct    points
	SysDiskUsageBytes points
	SysDiskUsagePct   points
	SysCPUUsagePct    points
	Qps               points
	Latency           points

	MinTime int64
	MaxTime int64
}

func handleDashboard(ss *serverState, rs *requestState) error {
	ctx := ss.ctx
	instance := rs.r.FormValue(paramDashboardName)
	if instance == "" {
		return fmt.Errorf("parameter %q required for instance name", paramDashboardName)
	}
	if isOwner, err := isOwnerOfInstance(rs.email, kubeNameFromMountName(instance)); err != nil {
		return err
	} else if !isOwner {
		return verror.New(verror.ErrNoExistOrNoAccess, ctx)
	}

	tmplArgs := struct {
		ServerName,
		Instance,
		Email string
	}{
		ServerName: ss.args.serverName,
		Instance:   instance,
		Email:      rs.email,
	}
	if err := ss.args.assets.executeTemplate(rs.w, dashboardTmpl, tmplArgs); err != nil {
		return fmt.Errorf("failed to render dashboard template: %v", err)
	}
	return nil
}

// handleStats responds to /stats request. It retrieves time series data
// for the given syncbase instance from GCM.
func handleStats(ss *serverState, rs *requestState) error {
	ctx := ss.ctx

	mountedName := rs.r.FormValue(paramDashboardName)
	if mountedName == "" {
		return fmt.Errorf("parameter %q required for instance name", paramDashboardName)
	}
	if isOwner, err := isOwnerOfInstance(rs.email, kubeNameFromMountName(mountedName)); err != nil {
		return err
	} else if !isOwner {
		return verror.New(verror.ErrNoExistOrNoAccess, ctx)
	}

	now := time.Now()
	if gcmService == nil || now.Sub(gcmLastAuthTime) > gcmAuthTimeout {
		s, err := gcm.Authenticate(ss.args.monitoringKeyFile)
		if err != nil {
			return err
		}
		gcmService = s
		gcmLastAuthTime = now
	}

	// Get duration (default to 1h) and instance mounted name.
	duration := defaultDuration
	if strDuration := rs.r.FormValue(paramDashbordDuration); strDuration != "" {
		d, err := strconv.ParseInt(strDuration, 10, 64)
		if err != nil {
			return err
		}
		duration = time.Duration(d) * time.Second
	}

	// Get data from GCM.
	md, err := gcm.GetMetric(ss.args.dashboardGCMMetric, ss.args.dashboardGCMProject)
	if err != nil {
		return err
	}
	filters := []string{
		fmt.Sprintf("metric.type=%q", md.Type),
		fmt.Sprintf("metric.label.mounted_name=%q", mountedName),
	}
	nextPageToken := ""
	tsMap := map[string]points{}
	for {
		listCall := gcmService.Projects.TimeSeries.List(fmt.Sprintf("projects/%s", ss.args.dashboardGCMProject)).
			IntervalStartTime(now.Add(-duration).UTC().Format(time.RFC3339)).
			IntervalEndTime(now.UTC().Format(time.RFC3339)).
			Filter(strings.Join(filters, " AND ")).
			PageToken(nextPageToken)
		alignmentPeriod := getAlignmentPeriodInSeconds(duration)
		if alignmentPeriod >= 0 {
			listCall = listCall.AggregationAlignmentPeriod(fmt.Sprintf("%ds", alignmentPeriod)).AggregationPerSeriesAligner("ALIGN_MEAN")
		}
		resp, err := listCall.Do()
		if err != nil {
			return err
		}
		for _, ts := range resp.TimeSeries {
			metricName := ts.Metric.Labels["metric_name"]
			for _, pt := range ts.Points {
				epochTime, err := time.Parse(time.RFC3339, pt.Interval.EndTime)
				if err != nil {
					ctx.Errorf("Parse(%s) failed: %v", pt.Interval.EndTime, err)
					continue
				}
				tsMap[metricName] = append(tsMap[metricName], point{
					Timestamp: epochTime.Unix(),
					Value:     pt.Value.DoubleValue,
				})
			}
		}
		nextPageToken = resp.NextPageToken
		if nextPageToken == "" {
			break
		}
	}

	// Process data and put it into statsResult.
	result := statsResult{}
	minTime := int64(math.MaxInt64)
	maxTime := int64(0)
	for metricName, pts := range tsMap {
		tsMap[metricName].Sort()
		if pts[0].Timestamp < minTime {
			minTime = pts[0].Timestamp
		}
		if pts[len(pts)-1].Timestamp > maxTime {
			maxTime = pts[len(pts)-1].Timestamp
		}
		switch metricName {
		case "sysmem-usage-bytes":
			result.SysMemUsageBytes = pts
		case "sysmem-usage-pct":
			result.SysMemUsagePct = pts
		case "sysdisk-usage-bytes":
			result.SysDiskUsageBytes = pts
		case "sysdisk-usage-pct":
			result.SysDiskUsagePct = pts
		case "syscpu-usage-pct":
			result.SysCPUUsagePct = pts
		case "latency":
			result.Latency = pts
		case "qps":
			result.Qps = pts
		}
	}
	result.MinTime = minTime
	result.MaxTime = maxTime

	// Convert result to json and return it.
	b, err := json.MarshalIndent(&result, "", "  ")
	if err != nil {
		return err
	}
	rs.w.Header().Set("Content-Type", "application/json")
	rs.w.Write(b)

	return nil
}

func getAlignmentPeriodInSeconds(duration time.Duration) int {
	switch {
	case duration <= time.Hour*12:
		return -1
	case duration <= time.Hour*24:
		return 5 * 60
	case duration <= time.Hour*24*7:
		return 20 * 60
	case duration <= time.Hour*24*30:
		return 30 * 60
	default:
		return 60 * 60
	}
}
