package projector

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

const Schema = "gooo.protected-change-gate-projector/v0.1"

type Decision string

const (
	Closed  Decision = "CLOSED"
	Unknown Decision = "UNKNOWN"
	Refuted Decision = "REFUTED"
)

var RequiredStages = []string{
	"AUTHOR_BRANCH",
	"OPEN_PR",
	"PR_ACTIONS_GREEN",
	"MERGE",
	"MAIN_ACTIONS_GREEN",
	"POLICY_LOCK",
	"ANNOTATED_TAG",
	"DRAFT_RELEASE",
	"UPLOAD_NEW_ASSETS",
	"VERIFY_TAG_TARGET_AND_ASSET_DIGEST",
	"PUBLISH",
	"IMMUTABLE_AUDIT",
}

type Receipt struct {
	ID             string            `json:"id"`
	Kind           string            `json:"kind"`
	CreatedAt      string            `json:"created_at,omitempty"`
	Ref            string            `json:"ref,omitempty"`
	SHA            string            `json:"sha,omitempty"`
	HeadSHA        string            `json:"head_sha,omitempty"`
	PRNumber       int               `json:"pr_number,omitempty"`
	ReleaseID      string            `json:"release_id,omitempty"`
	Tag            string            `json:"tag,omitempty"`
	TargetSHA      string            `json:"target_sha,omitempty"`
	AssetName      string            `json:"asset_name,omitempty"`
	AssetDigest    string            `json:"asset_digest,omitempty"`
	Status         string            `json:"status,omitempty"`
	Annotated      *bool             `json:"annotated,omitempty"`
	Draft          *bool             `json:"draft,omitempty"`
	Fresh          *bool             `json:"fresh,omitempty"`
	DirectMain     bool              `json:"direct_main,omitempty"`
	Substantive    bool              `json:"substantive,omitempty"`
	ChangeClass    string            `json:"change_class,omitempty"`
	Action         string            `json:"action,omitempty"`
	HistoricalShape string           `json:"historical_shape,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty"`
}

type UnknownFrontier struct {
	Stage         string `json:"stage"`
	Step          string `json:"step"`
	Reason        string `json:"reason"`
	UnknownClass  string `json:"unknown_class"`
	NextOperation string `json:"next_operation"`
	BlockedBy     string `json:"blocked_by"`
}

type Evidence struct {
	State         Decision `json:"state"`
	Stage         string   `json:"stage"`
	Step          string   `json:"step"`
	Reason        string   `json:"reason"`
	UnknownClass  string   `json:"unknown_class"`
	NextOperation string   `json:"next_operation"`
	BlockedBy     string   `json:"blocked_by"`
}

type Measurement struct {
	State     string `json:"state"`
	WallMS    *int64 `json:"wall_ms"`
	RSSBytes  *int64 `json:"rss_bytes"`
	Unknown   *Evidence `json:"unknown,omitempty"`
}

type Metrics struct {
	EventsObserved            int      `json:"events_observed"`
	GatesRequired             int      `json:"gates_required"`
	GatesClosed               int      `json:"gates_closed"`
	GatesUnknown              int      `json:"gates_unknown"`
	GatesRefuted              int      `json:"gates_refuted"`
	DirectMainEvents          int      `json:"direct_main_events"`
	RecreatedArtifactAttempts int      `json:"recreated_artifact_attempts"`
	AcceptedResumeOperations  int      `json:"accepted_resume_operations"`
	RepositoryWrites          int      `json:"repository_writes"`
	RemoteMutations           int      `json:"remote_mutations"`
	DestructiveOperations     int      `json:"destructive_operations"`
	GeneratedArtifacts        []string `json:"generated_artifacts"`
}

type OperationalRecord struct {
	ReceiptID  string `json:"receipt_id"`
	Event      string `json:"event"`
	Outcome    string `json:"outcome"`
	Resolution string `json:"resolution,omitempty"`
	Detail     string `json:"detail,omitempty"`
}

type Projection struct {
	Schema                  string              `json:"schema"`
	Decision                Decision            `json:"decision"`
	CurrentStage            string              `json:"current_stage"`
	NextOperation           string              `json:"next_operation"`
	MinimalCausalFrontier   []string            `json:"minimal_causal_frontier"`
	Reason                  string              `json:"reason"`
	Unknown                 *UnknownFrontier    `json:"unknown,omitempty"`
	Metrics                 Metrics             `json:"metrics"`
	OperationalHistory      []OperationalRecord `json:"operational_history"`
	UtilityImprovement      Evidence            `json:"utility_improvement"`
	Measurement             Measurement         `json:"measurement"`
}

type EventFile struct {
	Events []Receipt `json:"events"`
}

type Case struct {
	Name   string    `json:"name"`
	Events []Receipt `json:"events"`
	Expect Expectation `json:"expect"`
}

type FixtureFile struct {
	Cases []Case `json:"cases"`
}

type Expectation struct {
	Decision             Decision `json:"decision"`
	CurrentStage         string   `json:"current_stage"`
	GatesClosed          int      `json:"gates_closed"`
	GatesUnknown         int      `json:"gates_unknown"`
	GatesRefuted         int      `json:"gates_refuted"`
	DirectMainEvents     int      `json:"direct_main_events"`
	RecreatedAttempts    int      `json:"recreated_artifact_attempts"`
	AcceptedResumes      int      `json:"accepted_resume_operations"`
}

type Cell struct {
	ID       string
	Class    string
	Role     string
	Stage    string
	Activity string
}

type Activity struct {
	ID    string
	Cell  string
	Stage string
	On    string
	Emit  string
}

type Contract struct {
	Version                    string
	Authority                  string
	Mode                       string
	Precedence                 []Decision
	CellCount                  int
	ActivityCount              int
	CrossProjectRequiredGates  int
	Cells                      []Cell
	Activities                 []Activity
}

func LoadContract(r io.Reader) (Contract, error) {
	var contract Contract
	var section string
	var currentCell *Cell
	var currentActivity *Activity
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.Contains(strings.ToLower(line), "score") {
			return Contract{}, errors.New("scalar scoring is not part of the contract")
		}
		if strings.HasPrefix(line, "[cell ") {
			if currentCell != nil {
				contract.Cells = append(contract.Cells, *currentCell)
			}
			id, err := quotedHeaderID(line, "cell")
			if err != nil {
				return Contract{}, err
			}
			cell := Cell{ID: id}
			currentCell = &cell
			currentActivity = nil
			section = "cell"
			continue
		}
		if strings.HasPrefix(line, "[activity ") {
			if currentCell != nil {
				contract.Cells = append(contract.Cells, *currentCell)
				currentCell = nil
			}
			if currentActivity != nil {
				contract.Activities = append(contract.Activities, *currentActivity)
			}
			id, err := quotedHeaderID(line, "activity")
			if err != nil {
				return Contract{}, err
			}
			activity := Activity{ID: id}
			currentActivity = &activity
			section = "activity"
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return Contract{}, fmt.Errorf("invalid contract line: %s", line)
		}
		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), "\"")
		if section == "cell" && currentCell != nil {
			switch key {
			case "class": currentCell.Class = value
			case "role": currentCell.Role = value
			case "stage": currentCell.Stage = value
			case "activity": currentCell.Activity = value
			}
			continue
		}
		if section == "activity" && currentActivity != nil {
			switch key {
			case "cell": currentActivity.Cell = value
			case "stage": currentActivity.Stage = value
			case "on": currentActivity.On = value
			case "emit": currentActivity.Emit = value
			}
			continue
		}
		switch key {
		case "version": contract.Version = value
		case "authority": contract.Authority = value
		case "mode": contract.Mode = value
		case "precedence":
			for _, item := range strings.Split(value, ">") {
				contract.Precedence = append(contract.Precedence, Decision(strings.TrimSpace(item)))
			}
		case "cells": contract.CellCount, _ = strconv.Atoi(value)
		case "activities": contract.ActivityCount, _ = strconv.Atoi(value)
		case "cross_project_required_gates": contract.CrossProjectRequiredGates, _ = strconv.Atoi(value)
		}
	}
	if err := scanner.Err(); err != nil {
		return Contract{}, err
	}
	if currentCell != nil {
		contract.Cells = append(contract.Cells, *currentCell)
	}
	if currentActivity != nil {
		contract.Activities = append(contract.Activities, *currentActivity)
	}
	if err := ValidateContract(contract); err != nil {
		return Contract{}, err
	}
	return contract, nil
}

func quotedHeaderID(line, kind string) (string, error) {
	prefix := "[" + kind + " \""
	if !strings.HasPrefix(line, prefix) || !strings.HasSuffix(line, "\"]") {
		return "", fmt.Errorf("invalid %s header: %s", kind, line)
	}
	return strings.TrimSuffix(strings.TrimPrefix(line, prefix), "\"]"), nil
}

func ValidateContract(contract Contract) error {
	if contract.Version == "" || contract.Authority == "" || contract.Mode == "" {
		return errors.New("contract metadata is incomplete")
	}
	if contract.CellCount != 12 || len(contract.Cells) != 12 {
		return fmt.Errorf("contract requires exactly 12 cells, declared=%d actual=%d", contract.CellCount, len(contract.Cells))
	}
	if contract.ActivityCount != 12 || len(contract.Activities) != 12 {
		return fmt.Errorf("contract requires exactly 12 activities, declared=%d actual=%d", contract.ActivityCount, len(contract.Activities))
	}
	if contract.CrossProjectRequiredGates != 0 {
		return errors.New("cross-project required gates must be zero")
	}
	wantPrecedence := []Decision{Refuted, Unknown, Closed}
	if !equalDecisions(contract.Precedence, wantPrecedence) {
		return errors.New("precedence must be REFUTED > UNKNOWN > CLOSED")
	}
	classCount := map[string]int{}
	roleCount := map[string]int{}
	seenStages := make(map[string]bool)
	for i, cell := range contract.Cells {
		if cell.Stage != RequiredStages[i] {
			return fmt.Errorf("cell %s is out of order", cell.ID)
		}
		if cell.Activity == "" {
			return fmt.Errorf("cell %s has no activity", cell.ID)
		}
		classCount[cell.Class]++
		roleCount[cell.Role]++
		if seenStages[cell.Stage] {
			return fmt.Errorf("duplicate stage %s", cell.Stage)
		}
		seenStages[cell.Stage] = true
	}
	for _, class := range []string{"FOUNDATION", "COHERENCE", "REGRESSION"} {
		if classCount[class] != 4 {
			return fmt.Errorf("class %s must have four cells", class)
		}
	}
	for _, role := range []string{"DRIVER", "OUTCOME", "GUARDRAIL"} {
		if roleCount[role] != 4 {
			return fmt.Errorf("role %s must have four activities", role)
		}
	}
	activityByID := make(map[string]Activity)
	for _, activity := range contract.Activities {
		if _, exists := activityByID[activity.ID]; exists {
			return fmt.Errorf("duplicate activity %s", activity.ID)
		}
		activityByID[activity.ID] = activity
	}
	for _, cell := range contract.Cells {
		activity, ok := activityByID[cell.Activity]
		if !ok || activity.Cell != cell.ID || activity.Stage != cell.Stage || activity.On == "" || activity.Emit == "" {
			return fmt.Errorf("activity contract incomplete for %s", cell.ID)
		}
	}
	return nil
}

func equalDecisions(left, right []Decision) bool {
	if len(left) != len(right) { return false }
	for i := range left {
		if left[i] != right[i] { return false }
	}
	return true
}

type refutation struct {
	stage  string
	reason string
	ids    []string
}

func Project(events []Receipt) Projection {
	return project(events, nil)
}

func ProjectWithContract(events []Receipt, contract Contract) (Projection, error) {
	if err := ValidateContract(contract); err != nil {
		return Projection{}, err
	}
	return project(events, &contract), nil
}

func project(input []Receipt, contract *Contract) Projection {
	events, ambiguous := uniqueReceipts(input)
	metrics := Metrics{
		EventsObserved: len(events),
		GatesRequired: len(RequiredStages),
		RepositoryWrites: 0,
		RemoteMutations: 0,
		DestructiveOperations: 0,
		GeneratedArtifacts: []string{},
	}
	if contract != nil && contract.CrossProjectRequiredGates == 0 {
		metrics.GatesRequired = contract.CellCount
	}
	operational := operationalRecords(events)
	metrics.DirectMainEvents = countDirectMain(events)
	metrics.RecreatedArtifactAttempts = countRecreatedAttempts(events)
	if metrics.RecreatedArtifactAttempts > 0 {
		return makeRefuted(events, metrics, operational, "IMMUTABLE_AUDIT", "an immutable artifact replacement or recreation was attempted", idsForRecreated(events))
	}
	if len(metricsDirectMain(events)) > 0 {
		ids := []string{}
		for _, event := range metricsDirectMain(events) { ids = append(ids, event.ID) }
		return makeRefuted(events, metrics, operational, "OPEN_PR", "substantive repository change reached the default branch without a pull request", ids)
	}
	if len(ambiguous) > 0 {
		return makeUnknown(metrics, operational, 0, UnknownFrontier{
			Stage: "AUTHOR_BRANCH", Step: "accept immutable event receipts", Reason: "the same immutable ID has conflicting receipt bodies", UnknownClass: "ambiguous_receipt", NextOperation: "supply one canonical receipt for each immutable ID", BlockedBy: strings.Join(ambiguous, ","),
		})
	}
	indices := stageIndices(events)
	if bad := orderRefutation(events, indices); bad != nil {
		return makeRefuted(events, metrics, operational, bad.stage, bad.reason, bad.ids)
	}

	closed := 0
	branch := receiptAt(events, indices["AUTHOR_BRANCH"])
	if branch == nil {
		return makeUnknown(metrics, operational, closed, missingFrontier("AUTHOR_BRANCH", "record the author branch", "author_branch_receipt", "create and record an author branch", "no author branch receipt"))
	}
	closed++
	pr := receiptAt(events, indices["OPEN_PR"])
	if pr == nil {
		return makeUnknown(metrics, operational, closed, missingFrontier("OPEN_PR", "open the substantive pull request", "pull_request_receipt", "open a pull request", branch.ID))
	}
	closed++
	prActions := receiptAt(events, indices["PR_ACTIONS_GREEN"])
	if prActions == nil {
		return makeUnknown(metrics, operational, closed, missingFrontier("PR_ACTIONS_GREEN", "accept green pull-request checks", "missing_or_not_green_ci", "wait for the pull-request Actions run to be green", pr.ID))
	}
	if !green(prActions) {
		return makeUnknown(metrics, operational, closed, UnknownFrontier{Stage: "PR_ACTIONS_GREEN", Step: "accept green pull-request checks", Reason: "the pull-request check receipt is not green", UnknownClass: "ci_not_green", NextOperation: "wait for or rerun pull-request Actions", BlockedBy: prActions.ID})
	}
	closed++
	merge := receiptAt(events, indices["MERGE"])
	if merge == nil {
		return makeUnknown(metrics, operational, closed, missingFrontier("MERGE", "merge the green pull request", "merge_receipt", "merge the pull request after checks are green", prActions.ID))
	}
	closed++
	mainActions := receiptAt(events, indices["MAIN_ACTIONS_GREEN"])
	if mainActions == nil {
		return makeUnknown(metrics, operational, closed, missingFrontier("MAIN_ACTIONS_GREEN", "accept fresh green main checks", "missing_or_stale_ci", "wait for fresh main Actions validation", merge.ID))
	}
	if !green(mainActions) || (mainActions.Fresh != nil && !*mainActions.Fresh) || (merge.SHA != "" && mainActions.HeadSHA != "" && mainActions.HeadSHA != merge.SHA) {
		return makeUnknown(metrics, operational, closed, UnknownFrontier{Stage: "MAIN_ACTIONS_GREEN", Step: "accept fresh green main checks", Reason: "main validation is missing, not green, or stale for the merged commit", UnknownClass: "stale_receipt", NextOperation: "run or await main Actions for the merged commit", BlockedBy: mainActions.ID})
	}
	closed++
	policy := receiptAt(events, indices["POLICY_LOCK"])
	if policy == nil {
		return makeUnknown(metrics, operational, closed, missingFrontier("POLICY_LOCK", "apply the immutable release policy once", "missing_policy_receipt", "apply the policy lock before release plumbing", mainActions.ID))
	}
	closed++
	tag := receiptAt(events, indices["ANNOTATED_TAG"])
	if tag == nil {
		return makeUnknown(metrics, operational, closed, missingFrontier("ANNOTATED_TAG", "create the annotated tag", "missing_tag_receipt", "create one annotated tag for the merged commit", policy.ID))
	}
	if tag.Annotated == nil || !*tag.Annotated {
		return makeRefuted(events, metrics, operational, "ANNOTATED_TAG", "the release tag receipt is lightweight instead of annotated", []string{tag.ID})
	}
	closed++
	draft := receiptAt(events, indices["DRAFT_RELEASE"])
	if draft == nil || draft.ReleaseID == "" || (draft.Draft != nil && !*draft.Draft) {
		return makeUnknown(metrics, operational, closed, UnknownFrontier{Stage: "DRAFT_RELEASE", Step: "create or resume the draft release", Reason: "no exact draft release identity is available", UnknownClass: "missing_or_ambiguous_release_receipt", NextOperation: "create or resume one draft release by exact release ID", BlockedBy: tag.ID})
	}
	closed++
	resumeCount := acceptedResumes(events, draft.ReleaseID)
	metrics.AcceptedResumeOperations = resumeCount
	upload := receiptAt(events, indices["UPLOAD_NEW_ASSETS"])
	if upload == nil || upload.ReleaseID != draft.ReleaseID || upload.AssetName == "" || upload.AssetDigest == "" {
		return makeUnknown(metrics, operational, closed, missingFrontier("UPLOAD_NEW_ASSETS", "upload new assets to the exact draft release", "missing_asset_receipt", "upload new assets without replacing existing names", draft.ID))
	}
	closed++
	verify := receiptAt(events, indices["VERIFY_TAG_TARGET_AND_ASSET_DIGEST"])
	if verify == nil {
		return makeUnknown(metrics, operational, closed, missingFrontier("VERIFY_TAG_TARGET_AND_ASSET_DIGEST", "verify tag target and asset digest", "missing_verification_receipt", "verify the exact tag target and asset digest", upload.ID))
	}
	closed++
	publish := receiptAt(events, indices["PUBLISH"])
	if publish == nil || publish.ReleaseID != draft.ReleaseID {
		return makeUnknown(metrics, operational, closed, missingFrontier("PUBLISH", "publish the verified draft release", "missing_publish_receipt", "publish the exact verified draft release", verify.ID))
	}
	closed++
	audit := receiptAt(events, indices["IMMUTABLE_AUDIT"])
	if audit == nil || !green(audit) {
		return makeUnknown(metrics, operational, closed, UnknownFrontier{Stage: "IMMUTABLE_AUDIT", Step: "audit immutable history and assets", Reason: "no passing immutable audit receipt is available", UnknownClass: "missing_or_not_green_audit", NextOperation: "run the immutable audit without rewriting history", BlockedBy: publish.ID})
	}
	closed++
	metrics.GatesClosed = closed
	return Projection{
		Schema: Schema,
		Decision: Closed,
		CurrentStage: "IMMUTABLE_AUDIT",
		NextOperation: "none",
		MinimalCausalFrontier: []string{audit.ID},
		Reason: "all bounded change-authority gates are closed in order",
		Metrics: metrics,
		OperationalHistory: operational,
		UtilityImprovement: unknownEvidence("UTILITY_IMPROVEMENT", "assess external utility only with independent evidence", "external_utility_not_evidenced", "none", "no independent external utility evidence was supplied"),
		Measurement: unknownMeasurement(),
	}
}

func missingFrontier(stage, step, class, next, blocked string) UnknownFrontier {
	return UnknownFrontier{Stage: stage, Step: step, Reason: "the required receipt is missing or not yet authoritative", UnknownClass: class, NextOperation: next, BlockedBy: blocked}
}

func makeUnknown(metrics Metrics, operational []OperationalRecord, closed int, frontier UnknownFrontier) Projection {
	metrics.GatesClosed = closed
	metrics.GatesUnknown = 1
	return Projection{
		Schema: Schema,
		Decision: Unknown,
		CurrentStage: frontier.Stage,
		NextOperation: frontier.NextOperation,
		MinimalCausalFrontier: []string{frontier.BlockedBy},
		Reason: frontier.Reason,
		Unknown: &frontier,
		Metrics: metrics,
		OperationalHistory: operational,
		UtilityImprovement: unknownEvidence("UTILITY_IMPROVEMENT", "assess external utility only with independent evidence", "external_utility_not_evidenced", "none", "no independent external utility evidence was supplied"),
		Measurement: unknownMeasurement(),
	}
}

func makeRefuted(events []Receipt, metrics Metrics, operational []OperationalRecord, stage, reason string, ids []string) Projection {
	metrics.GatesClosed = closedBeforeStage(events, stage)
	metrics.GatesRefuted = 1
	return Projection{
		Schema: Schema,
		Decision: Refuted,
		CurrentStage: stage,
		NextOperation: "preserve the receipt and stop the unauthorized operation",
		MinimalCausalFrontier: ids,
		Reason: reason,
		Metrics: metrics,
		OperationalHistory: operational,
		UtilityImprovement: unknownEvidence("UTILITY_IMPROVEMENT", "assess external utility only with independent evidence", "external_utility_not_evidenced", "none", "no independent external utility evidence was supplied"),
		Measurement: unknownMeasurement(),
	}
}

func closedBeforeStage(events []Receipt, stage string) int {
	indices := stageIndices(events)
	stageIndex := indices[stage]
	closed := 0
	for _, candidate := range RequiredStages {
		if candidate == stage { return closed }
		if stageIndex >= 0 && indices[candidate] >= stageIndex { return closed }
		event := receiptAt(events, indices[candidate])
		if event == nil { return closed }
		switch candidate {
		case "PR_ACTIONS_GREEN":
			if !green(event) { return closed }
		case "MAIN_ACTIONS_GREEN":
			merge := receiptAt(events, indices["MERGE"])
			if !green(event) || (event.Fresh != nil && !*event.Fresh) || (merge != nil && merge.SHA != "" && event.HeadSHA != "" && event.HeadSHA != merge.SHA) { return closed }
		case "ANNOTATED_TAG":
			if event.Annotated == nil || !*event.Annotated { return closed }
		case "DRAFT_RELEASE":
			if event.ReleaseID == "" || (event.Draft != nil && !*event.Draft) { return closed }
		case "UPLOAD_NEW_ASSETS":
			if event.ReleaseID == "" || event.AssetName == "" || event.AssetDigest == "" { return closed }
		case "VERIFY_TAG_TARGET_AND_ASSET_DIGEST":
			upload := receiptAt(events, indices["UPLOAD_NEW_ASSETS"])
			if upload == nil || event.ReleaseID != upload.ReleaseID || event.AssetName != upload.AssetName || event.AssetDigest != upload.AssetDigest { return closed }
		case "PUBLISH":
			if event.ReleaseID == "" { return closed }
		case "IMMUTABLE_AUDIT":
			if !green(event) { return closed }
		}
		closed++
	}
	return closed
}

func unknownEvidence(stage, step, class, next, reason string) Evidence {
	return Evidence{State: Unknown, Stage: stage, Step: step, Reason: reason, UnknownClass: class, NextOperation: next, BlockedBy: "independent evidence"}
}

func unknownMeasurement() Measurement {
	evidence := unknownEvidence("MEASUREMENT", "observe wall time and RSS", "not_observed", "run the measurement step in GitHub Actions", "wall/RSS were not observed by the projector")
	return Measurement{State: string(Unknown), WallMS: nil, RSSBytes: nil, Unknown: &evidence}
}

func uniqueReceipts(input []Receipt) ([]Receipt, []string) {
	seen := make(map[string][]byte)
	unique := make([]Receipt, 0, len(input))
	ambiguous := []string{}
	for _, receipt := range input {
		body, _ := json.Marshal(receipt)
		if prior, ok := seen[receipt.ID]; ok {
			if !bytes.Equal(prior, body) && receipt.ID != "" {
				ambiguous = append(ambiguous, receipt.ID)
			}
			continue
		}
		seen[receipt.ID] = body
		unique = append(unique, receipt)
	}
	return unique, ambiguous
}

func kind(receipt Receipt) string {
	return strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(receipt.Kind, "-", "_"), " ", "_"))
}

func hasKind(receipt Receipt, names ...string) bool {
	k := kind(receipt)
	for _, name := range names {
		if k == name { return true }
	}
	return false
}

func isDirectMain(receipt Receipt) bool {
	if receipt.DirectMain { return true }
	k := kind(receipt)
	mainRef := receipt.Ref == "main" || receipt.Ref == "refs/heads/main"
	change := receipt.Substantive || receipt.ChangeClass == "implementation" || receipt.ChangeClass == "maintenance" || receipt.ChangeClass == "release_plumbing"
	return mainRef && change && (k == "push" || k == "commit" || k == "change" || strings.Contains(k, "main"))
}

func metricsDirectMain(events []Receipt) []Receipt {
	result := []Receipt{}
	for _, event := range events {
		if isDirectMain(event) { result = append(result, event) }
	}
	return result
}

func countDirectMain(events []Receipt) int { return len(metricsDirectMain(events)) }

func countRecreatedAttempts(events []Receipt) int {
	count := 0
	seenAssets := make(map[string]bool)
	for _, event := range events {
		if hasKind(event, "asset_replaced", "asset_recreated", "release_recreated", "tag_replaced") || strings.EqualFold(event.Action, "replace") || strings.EqualFold(event.Action, "recreate") {
			count++
		}
		if hasKind(event, "asset_uploaded") && event.AssetName != "" {
			key := event.ReleaseID + "\x00" + event.AssetName
			if seenAssets[key] { count++ }
			seenAssets[key] = true
		}
	}
	return count
}

func idsForRecreated(events []Receipt) []string {
	ids := []string{}
	seenAssets := make(map[string]bool)
	for _, event := range events {
		if hasKind(event, "asset_replaced", "asset_recreated", "release_recreated", "tag_replaced") || strings.EqualFold(event.Action, "replace") || strings.EqualFold(event.Action, "recreate") {
			ids = append(ids, event.ID)
		}
		if hasKind(event, "asset_uploaded") && event.AssetName != "" {
			key := event.ReleaseID + "\x00" + event.AssetName
			if seenAssets[key] { ids = append(ids, event.ID) }
			seenAssets[key] = true
		}
	}
	return ids
}

func stageIndices(events []Receipt) map[string]int {
	indices := make(map[string]int, len(RequiredStages))
	for _, stage := range RequiredStages { indices[stage] = -1 }
	for i, event := range events {
		k := kind(event)
		switch {
		case hasKind(event, "author_branch", "branch_created"): setFirst(indices, "AUTHOR_BRANCH", i)
		case hasKind(event, "pull_request_opened", "pr_opened", "open_pr"): setFirst(indices, "OPEN_PR", i)
		case hasKind(event, "pr_actions", "pr_actions_green", "pull_request_actions"): setFirst(indices, "PR_ACTIONS_GREEN", i)
		case hasKind(event, "merge", "pull_request_merged"): setFirst(indices, "MERGE", i)
		case hasKind(event, "main_actions", "main_actions_green", "main_validation"): setFirst(indices, "MAIN_ACTIONS_GREEN", i)
		case hasKind(event, "policy_lock", "policy_mutation"): setFirst(indices, "POLICY_LOCK", i)
		case hasKind(event, "tag_created", "annotated_tag", "tag"): setFirst(indices, "ANNOTATED_TAG", i)
		case hasKind(event, "draft_release", "release_draft_created"): setFirst(indices, "DRAFT_RELEASE", i)
		case hasKind(event, "asset_uploaded", "release_asset_uploaded"): setFirst(indices, "UPLOAD_NEW_ASSETS", i)
		case hasKind(event, "asset_verified", "release_verified", "verification"): setFirst(indices, "VERIFY_TAG_TARGET_AND_ASSET_DIGEST", i)
		case hasKind(event, "release_published", "publish"): setFirst(indices, "PUBLISH", i)
		case hasKind(event, "immutable_audit", "audit"): setFirst(indices, "IMMUTABLE_AUDIT", i)
		case k == "":
		}
	}
	return indices
}

func setFirst(indices map[string]int, stage string, index int) { if indices[stage] < 0 { indices[stage] = index } }

func receiptAt(events []Receipt, index int) *Receipt {
	if index < 0 || index >= len(events) { return nil }
	return &events[index]
}

func stageIDs(events []Receipt, indices map[string]int, stages ...string) []string {
	ids := []string{}
	for _, stage := range stages {
		if event := receiptAt(events, indices[stage]); event != nil { ids = append(ids, event.ID) }
	}
	return ids
}

func orderRefutation(events []Receipt, indices map[string]int) *refutation {
	if indices["MERGE"] >= 0 && indices["PR_ACTIONS_GREEN"] >= 0 && indices["MERGE"] < indices["PR_ACTIONS_GREEN"] {
		return &refutation{"MERGE", "the pull request was merged before pull-request Actions became green", stageIDs(events, indices, "MERGE", "PR_ACTIONS_GREEN")}
	}
	if indices["MAIN_ACTIONS_GREEN"] >= 0 && indices["MERGE"] >= 0 && indices["MAIN_ACTIONS_GREEN"] < indices["MERGE"] {
		return &refutation{"MAIN_ACTIONS_GREEN", "main validation occurred before the pull-request merge", stageIDs(events, indices, "MAIN_ACTIONS_GREEN", "MERGE")}
	}
	policyCount := 0
	for _, event := range events { if hasKind(event, "policy_lock", "policy_mutation") { policyCount++ } }
	if policyCount > 1 {
		return &refutation{"POLICY_LOCK", "immutable policy mutation occurred more than once", policyIDs(events)}
	}
	if indices["ANNOTATED_TAG"] >= 0 && indices["POLICY_LOCK"] >= 0 && indices["ANNOTATED_TAG"] < indices["POLICY_LOCK"] {
		return &refutation{"POLICY_LOCK", "tagging began before the one-time policy lock", stageIDs(events, indices, "ANNOTATED_TAG", "POLICY_LOCK")}
	}
	if indices["DRAFT_RELEASE"] >= 0 && indices["ANNOTATED_TAG"] >= 0 && indices["DRAFT_RELEASE"] < indices["ANNOTATED_TAG"] {
		return &refutation{"DRAFT_RELEASE", "draft release creation occurred before the annotated tag", stageIDs(events, indices, "DRAFT_RELEASE", "ANNOTATED_TAG")}
	}
	if indices["UPLOAD_NEW_ASSETS"] >= 0 && indices["DRAFT_RELEASE"] >= 0 && indices["UPLOAD_NEW_ASSETS"] < indices["DRAFT_RELEASE"] {
		return &refutation{"UPLOAD_NEW_ASSETS", "asset upload occurred before the draft release", stageIDs(events, indices, "UPLOAD_NEW_ASSETS", "DRAFT_RELEASE")}
	}
	if indices["VERIFY_TAG_TARGET_AND_ASSET_DIGEST"] >= 0 && indices["UPLOAD_NEW_ASSETS"] >= 0 && indices["VERIFY_TAG_TARGET_AND_ASSET_DIGEST"] < indices["UPLOAD_NEW_ASSETS"] {
		return &refutation{"VERIFY_TAG_TARGET_AND_ASSET_DIGEST", "asset verification occurred before asset upload", stageIDs(events, indices, "VERIFY_TAG_TARGET_AND_ASSET_DIGEST", "UPLOAD_NEW_ASSETS")}
	}
	if publish := receiptAt(events, indices["PUBLISH"]); publish != nil {
		if indices["POLICY_LOCK"] < 0 || indices["PUBLISH"] < indices["POLICY_LOCK"] {
			return &refutation{"PUBLISH", "release publication occurred before the one-time policy lock", stageIDs(events, indices, "PUBLISH", "POLICY_LOCK")}
		}
		if indices["VERIFY_TAG_TARGET_AND_ASSET_DIGEST"] < 0 || indices["PUBLISH"] < indices["VERIFY_TAG_TARGET_AND_ASSET_DIGEST"] {
			return &refutation{"PUBLISH", "release publication occurred before tag-target and asset-digest verification", stageIDs(events, indices, "PUBLISH", "VERIFY_TAG_TARGET_AND_ASSET_DIGEST")}
		}
		if indices["DRAFT_RELEASE"] < 0 || indices["PUBLISH"] < indices["DRAFT_RELEASE"] {
			return &refutation{"PUBLISH", "release publication occurred without a prior draft release", []string{publish.ID}}
		}
	}
	tag := receiptAt(events, indices["ANNOTATED_TAG"])
	merge := receiptAt(events, indices["MERGE"])
	if tag != nil && tag.Annotated != nil && *tag.Annotated && tag.TargetSHA != "" && merge != nil && merge.SHA != "" && tag.TargetSHA != merge.SHA {
		return &refutation{"VERIFY_TAG_TARGET_AND_ASSET_DIGEST", "annotated tag target does not equal the merged commit", []string{tag.ID, merge.ID}}
	}
	upload := receiptAt(events, indices["UPLOAD_NEW_ASSETS"])
	verify := receiptAt(events, indices["VERIFY_TAG_TARGET_AND_ASSET_DIGEST"])
	if upload != nil && verify != nil && (upload.ReleaseID != verify.ReleaseID || upload.AssetName != verify.AssetName || upload.AssetDigest != verify.AssetDigest) {
		return &refutation{"VERIFY_TAG_TARGET_AND_ASSET_DIGEST", "verified asset identity or digest differs from the uploaded asset", []string{upload.ID, verify.ID}}
	}
	return nil
}

func policyIDs(events []Receipt) []string {
	ids := []string{}
	for _, event := range events { if hasKind(event, "policy_lock", "policy_mutation") { ids = append(ids, event.ID) } }
	return ids
}

func green(event *Receipt) bool {
	return strings.EqualFold(event.Status, "green") || strings.EqualFold(event.Status, "success") || strings.EqualFold(event.Status, "passed")
}

func acceptedResumes(events []Receipt, releaseID string) int {
	count := 0
	for _, event := range events {
		if hasKind(event, "resume", "release_resumed") && event.ReleaseID == releaseID && event.ReleaseID != "" && !strings.EqualFold(event.Action, "recreate") && !strings.EqualFold(event.Action, "replace") {
			count++
		}
	}
	return count
}

func operationalRecords(events []Receipt) []OperationalRecord {
	records := []OperationalRecord{}
	for _, event := range events {
		if hasKind(event, "draft_lookup", "release_lookup") && strings.Contains(event.Status, "404") {
			records = append(records, OperationalRecord{ReceiptID: event.ID, Event: "transient_draft_lookup_404", Outcome: "preserved_operational_provenance", Resolution: "exact_release_id", Detail: "the endpoint failure did not override an exact draft release identity"})
		}
	}
	return records
}

func DecodeEvents(data []byte) ([]Receipt, error) {
	var file EventFile
	if err := json.Unmarshal(data, &file); err == nil && file.Events != nil {
		return file.Events, nil
	}
	var events []Receipt
	if err := json.Unmarshal(data, &events); err != nil {
		return nil, err
	}
	return events, nil
}

func DecodeFixtures(data []byte) (FixtureFile, error) {
	var fixtures FixtureFile
	if err := json.Unmarshal(data, &fixtures); err != nil { return FixtureFile{}, err }
	return fixtures, nil
}

func ReadContractFile(path string) (Contract, error) {
	file, err := os.Open(path)
	if err != nil { return Contract{}, err }
	defer file.Close()
	return LoadContract(file)
}
