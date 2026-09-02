package projector

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConformanceFixtureVectors(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "fixtures", "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	fixtures, err := DecodeFixtures(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(fixtures.Cases) != 12 {
		t.Fatalf("want exactly 12 fixture cases, got %d", len(fixtures.Cases))
	}
	if !equalCounts(fixtures.ProofChoiceCounts, map[string]int{"FOUNDATION": 4, "COHERENCE": 4, "REGRESSION": 4}) {
		t.Fatal("fixture proof_choice_counts must be exactly 4/4/4")
	}
	if !equalCounts(fixtures.IndicatorCounts, map[string]int{"DRIVER": 4, "OUTCOME": 4, "GUARDRAIL": 4}) {
		t.Fatal("fixture indicator_counts must be exactly 4/4/4")
	}
	if len(fixtures.OperationalHistory) != 10 {
		t.Fatalf("want 10 explicit operational-history records, got %d", len(fixtures.OperationalHistory))
	}
	for _, record := range fixtures.OperationalHistory {
		if record.StableID == "" || record.Classification != "OPERATIONAL_REFUTED" {
			t.Fatal("operational history requires stable_id and classification=OPERATIONAL_REFUTED")
		}
	}
	for _, fixture := range fixtures.Cases {
		t.Run(fixture.Name, func(t *testing.T) {
			projection := Project(fixture.Events)
			if fixture.ProofChoice == "" || fixture.Indicator == "" {
				t.Fatal("every fixture case must emit literal proof_choice and indicator")
			}
			if !equalCounts(projection.ProofChoiceCounts, fixtures.ProofChoiceCounts) || !equalCounts(projection.IndicatorCounts, fixtures.IndicatorCounts) {
				t.Fatal("projection must emit exact top-level proof and indicator counts")
			}
			if projection.ProofChoice == "" || projection.Indicator == "" {
				t.Fatal("projection must emit literal proof_choice and indicator")
			}
			if projection.Decision != fixture.Expect.Decision {
				t.Fatalf("decision: want %s got %s", fixture.Expect.Decision, projection.Decision)
			}
			if projection.CurrentStage != fixture.Expect.CurrentStage {
				t.Fatalf("stage: want %s got %s", fixture.Expect.CurrentStage, projection.CurrentStage)
			}
			if projection.Metrics.GatesClosed != fixture.Expect.GatesClosed {
				t.Fatalf("gates_closed: want %d got %d", fixture.Expect.GatesClosed, projection.Metrics.GatesClosed)
			}
			if projection.Metrics.GatesUnknown != fixture.Expect.GatesUnknown {
				t.Fatalf("gates_unknown: want %d got %d", fixture.Expect.GatesUnknown, projection.Metrics.GatesUnknown)
			}
			if projection.Metrics.GatesRefuted != fixture.Expect.GatesRefuted {
				t.Fatalf("gates_refuted: want %d got %d", fixture.Expect.GatesRefuted, projection.Metrics.GatesRefuted)
			}
			if projection.Metrics.DirectMainEvents != fixture.Expect.DirectMainEvents {
				t.Fatalf("direct_main_events: want %d got %d", fixture.Expect.DirectMainEvents, projection.Metrics.DirectMainEvents)
			}
			if projection.Metrics.RecreatedArtifactAttempts != fixture.Expect.RecreatedAttempts {
				t.Fatalf("recreated_artifact_attempts: want %d got %d", fixture.Expect.RecreatedAttempts, projection.Metrics.RecreatedArtifactAttempts)
			}
			if projection.Metrics.AcceptedResumeOperations != fixture.Expect.AcceptedResumes {
				t.Fatalf("accepted_resume_operations: want %d got %d", fixture.Expect.AcceptedResumes, projection.Metrics.AcceptedResumeOperations)
			}
			if projection.Metrics.RepositoryWrites != 0 || projection.Metrics.RemoteMutations != 0 || projection.Metrics.DestructiveOperations != 0 {
				t.Fatal("projector must be read-only")
			}
			if projection.Decision == Unknown {
				if projection.Unknown == nil || projection.Unknown.Stage == "" || projection.Unknown.Step == "" || projection.Unknown.Reason == "" || projection.Unknown.UnknownClass == "" || projection.Unknown.NextOperation == "" || projection.Unknown.BlockedBy == "" {
					t.Fatal("UNKNOWN must include a complete causal frontier")
				}
			}
		})
	}
}

func TestMetacodeContractIsExecutableAndBounded(t *testing.T) {
	contract, err := ReadContractFile(filepath.Join("..", ".gooo"))
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateContract(contract); err != nil {
		t.Fatal(err)
	}
	if contract.CellCount != 12 || len(contract.Cells) != 12 {
		t.Fatal("contract cell count is not exactly 12")
	}
	if contract.ActivityCount != 12 || len(contract.Activities) != 12 {
		t.Fatal("contract activity count is not exactly 12")
	}
	if !equalCounts(contract.ProofChoiceCounts, map[string]int{"FOUNDATION": 4, "COHERENCE": 4, "REGRESSION": 4}) {
		t.Fatal("contract proof_choice_counts is not exactly 4/4/4")
	}
	if !equalCounts(contract.IndicatorCounts, map[string]int{"DRIVER": 4, "OUTCOME": 4, "GUARDRAIL": 4}) {
		t.Fatal("contract indicator_counts is not exactly 4/4/4")
	}
	for _, cell := range contract.Cells {
		if cell.ProofChoice == "" || cell.Indicator == "" {
			t.Fatalf("cell %s has incomplete literal evidence", cell.ID)
		}
	}
	projection, err := ProjectWithContract(nil, contract)
	if err != nil {
		t.Fatal(err)
	}
	if projection.Decision != Unknown || projection.CurrentStage != "AUTHOR_BRANCH" {
		t.Fatal("empty event stream must stop at an UNKNOWN author-branch frontier")
	}
}

func TestRefutedPrecedesUnknown(t *testing.T) {
	projection := Project([]Receipt{{ID: "direct", Kind: "push", Ref: "refs/heads/main", DirectMain: true, Substantive: true, ChangeClass: "implementation"}})
	if projection.Decision != Refuted {
		t.Fatalf("want REFUTED, got %s", projection.Decision)
	}
	if projection.Metrics.GatesUnknown != 0 {
		t.Fatal("known refutation must outrank unknown")
	}
}

func TestExactResumePreservesOperational404(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "fixtures", "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	fixtures, err := DecodeFixtures(data)
	if err != nil {
		t.Fatal(err)
	}
	for _, fixture := range fixtures.Cases {
		if fixture.Name != "interrupted_release_resumed_by_exact_release_id_without_recreation" {
			continue
		}
		projection := Project(fixture.Events)
		if projection.Decision != Closed {
			t.Fatalf("want CLOSED, got %s", projection.Decision)
		}
		if projection.Metrics.AcceptedResumeOperations != 1 {
			t.Fatal("exact resume was not accepted")
		}
		if len(projection.OperationalHistory) != 1 || projection.OperationalHistory[0].Outcome != "preserved_operational_provenance" {
			t.Fatal("transient 404 provenance was not preserved")
		}
		return
	}
	t.Fatal("resume fixture not found")
}
