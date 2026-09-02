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
	events, err := projector.DecodeEvents(data)
	if err != nil { fail("decode events: %v", err) }

	started := time.Now()
	projection, err := projector.ProjectWithContract(events, contract)
	if err != nil { fail("project events: %v", err) }
	projection.Metrics.GeneratedArtifacts = append([]string{}, generated...)
	if *measure {
		wall := time.Since(started).Milliseconds()
		projection.Measurement.State = "OBSERVED"
		projection.Measurement.WallMS = &wall
		if rss, ok := rssBytes(); ok {
			projection.Measurement.RSSBytes = &rss
			projection.Measurement.Unknown = nil
		} else {
			projection.Measurement.Unknown = &projector.Evidence{State: projector.Unknown, Stage: "MEASUREMENT", Step: "observe RSS", Reason: "the host did not expose a readable resident-set metric", UnknownClass: "rss_not_observed", NextOperation: "preserve null RSS", BlockedBy: "host metric"}
		}
	}

	encoded, err := json.MarshalIndent(projection, "", "  ")
	if err != nil { fail("encode projection: %v", err) }
	encoded = append(encoded, '\n')
	if *outPath == "" {
		_, err = os.Stdout.Write(encoded)
	} else {
		err = os.WriteFile(*outPath, encoded, 0o600)
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
