package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/kimjooyoon/gooo-protected-change-gate-projector/projector"
)

type stringList []string

type fixtureReplay struct {
	Schema      string                       `json:"schema"`
	Cases       []fixtureReplayCase          `json:"cases"`
	Measurement *projector.Measurement       `json:"measurement,omitempty"`
}

type fixtureReplayCase struct {
	Name       string              `json:"name"`
	Projection projector.Projection `json:"projection"`
}

func (list *stringList) String() string { return strings.Join(*list, ",") }

func (list *stringList) Set(value string) error {
	*list = append(*list, value)
	return nil
}

func main() {
	eventsPath := flag.String("events", "", "JSON event receipt file")
	contractPath := flag.String("metacode", ".gooo", "bounded Gooo metacode contract")
	outPath := flag.String("out", "", "caller-owned output path; stdout when empty")
	measure := flag.Bool("measure", false, "include wall time and RSS observed by this run")
	var generated stringList
	flag.Var(&generated, "generated-artifact", "generated artifact name to include in evidence; repeatable")
	flag.Parse()
	if *eventsPath == "" {
		fail("-events is required")
	}

	contract, err := projector.ReadContractFile(*contractPath)
	if err != nil { fail("read metacode: %v", err) }
	data, err := os.ReadFile(*eventsPath)
	if err != nil { fail("read events: %v", err) }

	started := time.Now()
	if fixtures, fixtureErr := projector.DecodeFixtures(data); fixtureErr == nil && fixtures.Cases != nil {
		replay := fixtureReplay{Schema: projector.Schema, Cases: make([]fixtureReplayCase, 0, len(fixtures.Cases))}
		for _, fixture := range fixtures.Cases {
			projection, projectErr := projector.ProjectWithContract(fixture.Events, contract)
			if projectErr != nil { fail("project fixture %s: %v", fixture.Name, projectErr) }
			projection.Metrics.GeneratedArtifacts = append([]string{}, generated...)
			replay.Cases = append(replay.Cases, fixtureReplayCase{Name: fixture.Name, Projection: projection})
		}
		if *measure {
			measurement := observedMeasurement(started)
			replay.Measurement = &measurement
		}
		writeJSON(replay, *outPath)
		return
	}

	events, err := projector.DecodeEvents(data)
	if err != nil { fail("decode events: %v", err) }
	projection, err := projector.ProjectWithContract(events, contract)
	if err != nil { fail("project events: %v", err) }
	projection.Metrics.GeneratedArtifacts = append([]string{}, generated...)
	if *measure { projection.Measurement = observedMeasurement(started) }
	writeJSON(projection, *outPath)
}

func observedMeasurement(started time.Time) projector.Measurement {
	wall := time.Since(started).Milliseconds()
	measurement := projector.Measurement{State: "OBSERVED", WallMS: &wall}
	if rss, ok := rssBytes(); ok {
		measurement.RSSBytes = &rss
		return measurement
	}
	measurement.Unknown = &projector.Evidence{State: projector.Unknown, Stage: "MEASUREMENT", Step: "observe RSS", Reason: "the host did not expose a readable resident-set metric", UnknownClass: "rss_not_observed", NextOperation: "preserve null RSS", BlockedBy: "host metric"}
	return measurement
}

func writeJSON(value any, outPath string) {
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil { fail("encode projection: %v", err) }
	encoded = append(encoded, '\n')
	if outPath == "" {
		_, err = os.Stdout.Write(encoded)
	} else {
		err = os.WriteFile(outPath, encoded, 0o600)
	}
	if err != nil { fail("write projection: %v", err) }
}

func rssBytes() (int64, bool) {
	data, err := os.ReadFile("/proc/self/status")
	if err != nil { return 0, false }
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 3 && fields[0] == "VmRSS:" {
			value, err := strconv.ParseInt(fields[1], 10, 64)
			if err == nil { return value * 1024, true }
		}
	}
	return int64(runtime.NumGoroutine()), false
}

func fail(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
