package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	goalx "github.com/vonbai/goalx"
)

const successCompilerVersion = "compiler-v2"

type bootstrapCompilerSources struct {
	Query            *MemoryQuery
	Context          *MemoryContext
	Intake           *RunIntake
	PolicySourceRefs []string
	PriorEntryIDs    []string
	SourceSlots      []CompilerInputSlot
	RejectedPriors   []CompilerRejectedPrior
}

func EnsureSuccessCompilation(projectRoot, runDir string, cfg *goalx.Config, meta *RunMetadata) error {
	if cfg == nil {
		return fmt.Errorf("run config is nil")
	}
	if err := RefreshRunMemoryContext(runDir); err != nil {
		return fmt.Errorf("refresh run memory context: %w", err)
	}

	goalState, err := EnsureGoalState(runDir)
	if err != nil {
		return fmt.Errorf("ensure goal state: %w", err)
	}
	objectiveContract, err := EnsureObjectiveContract(runDir, cfg.Objective)
	if err != nil {
		return fmt.Errorf("ensure objective contract: %w", err)
	}
	acceptanceState, err := EnsureAcceptanceState(runDir, cfg, goalState.Version)
	if err != nil {
		return fmt.Errorf("ensure acceptance state: %w", err)
	}

	objectiveContractHash, err := hashFileSHA256(ObjectiveContractPath(runDir))
	if err != nil {
		return err
	}
	goalHash, err := hashFileSHA256(GoalPath(runDir))
	if err != nil {
		return err
	}
	compilerSources, err := buildBootstrapCompilerSources(projectRoot, runDir)
	if err != nil {
		return err
	}
	compilerInput := compileBootstrapCompilerInput(runDir, compilerSources)
	if err := SaveCompilerInput(CompilerInputPath(runDir), compilerInput); err != nil {
		return err
	}

	successModel := compileBootstrapSuccessModel(cfg, objectiveContract, goalState, objectiveContractHash, goalHash, compilerSources)
	if err := SaveSuccessModel(SuccessModelPath(runDir), successModel); err != nil {
		return err
	}
	proofPlan := compileBootstrapProofPlan(goalState, acceptanceState, successModel)
	if err := SaveProofPlan(ProofPlanPath(runDir), proofPlan); err != nil {
		return err
	}
	workflowPlan := compileBootstrapWorkflowPlan()
	if err := SaveWorkflowPlan(WorkflowPlanPath(runDir), workflowPlan); err != nil {
		return err
	}
	domainPack, err := compileBootstrapDomainPack(cfg, meta, compilerSources)
	if err != nil {
		return err
	}
	if err := SaveDomainPack(DomainPackPath(runDir), domainPack); err != nil {
		return err
	}
	if err := SaveCompilerReport(CompilerReportPath(runDir), compileBootstrapCompilerReport(compilerSources)); err != nil {
		return err
	}
	return nil
}

func RefreshRunSuccessContextForRun(projectRoot, runDir string) (bool, error) {
	cfg, err := LoadRunSpec(runDir)
	if err != nil {
		return false, err
	}
	meta, err := LoadRunMetadata(RunMetadataPath(runDir))
	if err != nil {
		return false, err
	}
	if meta == nil {
		meta = &RunMetadata{
			Version:     1,
			ProjectRoot: projectRoot,
		}
	}
	return RefreshRunSuccessContext(projectRoot, runDir, cfg, meta)
}

func RefreshRunSuccessContext(projectRoot, runDir string, cfg *goalx.Config, meta *RunMetadata) (bool, error) {
	if cfg == nil {
		return false, fmt.Errorf("run config is nil")
	}
	if meta == nil {
		loaded, err := LoadRunMetadata(RunMetadataPath(runDir))
		if err != nil {
			return false, err
		}
		if loaded != nil {
			meta = loaded
		} else {
			meta = &RunMetadata{
				Version:     1,
				ProjectRoot: projectRoot,
			}
		}
	}

	beforeContext, err := LoadMemoryContextFile(MemoryContextPath(runDir))
	if err != nil {
		return false, err
	}
	beforeCompilerInput, err := LoadCompilerInput(CompilerInputPath(runDir))
	if err != nil {
		return false, err
	}
	beforeDomainPack, err := LoadDomainPack(DomainPackPath(runDir))
	if err != nil {
		return false, err
	}

	if !fileExists(SuccessModelPath(runDir)) || !fileExists(ProofPlanPath(runDir)) || !fileExists(WorkflowPlanPath(runDir)) {
		if err := EnsureSuccessCompilation(projectRoot, runDir, cfg, meta); err != nil {
			return false, err
		}
	} else {
		if err := RefreshRunMemoryContext(runDir); err != nil {
			return false, fmt.Errorf("refresh run memory context: %w", err)
		}
		compilerSources, err := buildBootstrapCompilerSources(projectRoot, runDir)
		if err != nil {
			return false, err
		}
		compilerInput := compileBootstrapCompilerInput(runDir, compilerSources)
		if err := SaveCompilerInput(CompilerInputPath(runDir), compilerInput); err != nil {
			return false, err
		}
		domainPack, err := compileBootstrapDomainPack(cfg, meta, compilerSources)
		if err != nil {
			return false, err
		}
		if err := SaveDomainPack(DomainPackPath(runDir), domainPack); err != nil {
			return false, err
		}
		if err := SaveCompilerReport(CompilerReportPath(runDir), compileBootstrapCompilerReport(compilerSources)); err != nil {
			return false, err
		}
	}

	afterContext, err := LoadMemoryContextFile(MemoryContextPath(runDir))
	if err != nil {
		return false, err
	}
	afterCompilerInput, err := LoadCompilerInput(CompilerInputPath(runDir))
	if err != nil {
		return false, err
	}
	afterDomainPack, err := LoadDomainPack(DomainPackPath(runDir))
	if err != nil {
		return false, err
	}
	return compilerInputSignature(beforeCompilerInput) != compilerInputSignature(afterCompilerInput) ||
		!stringSliceEqual(domainPackPriorIDs(beforeDomainPack), domainPackPriorIDs(afterDomainPack)) ||
		(!stringSliceEqual(successPriorStatements(beforeContext), successPriorStatements(afterContext)) && compilerInputSignature(afterCompilerInput) == ""), nil
}

func compileBootstrapSuccessModel(cfg *goalx.Config, objectiveContract *ObjectiveContract, goalState *GoalState, objectiveContractHash, goalHash string, sources *bootstrapCompilerSources) *SuccessModel {
	model := &SuccessModel{
		Version:               1,
		CompilerVersion:       successCompilerVersion,
		ObjectiveContractHash: objectiveContractHash,
		GoalHash:              goalHash,
		Dimensions: []SuccessDimension{
			{
				ID:       "dim-objective",
				Kind:     "objective",
				Text:     strings.TrimSpace(cfg.Objective),
				Required: true,
			},
		},
		CloseoutRequirements: []string{
			"all_required_goal_items_resolved",
			"proof_plan_coverage_ready",
			"workflow_gates_present",
		},
	}
	if objectiveContract != nil && strings.TrimSpace(objectiveContract.ObjectiveHash) != "" {
		model.CloseoutRequirements = append(model.CloseoutRequirements, "objective_contract_present")
	}
	if goalState != nil {
		for _, item := range goalState.Required {
			if strings.TrimSpace(item.ID) == "" || strings.TrimSpace(item.Text) == "" {
				continue
			}
			model.Dimensions = append(model.Dimensions, SuccessDimension{
				ID:       item.ID,
				Kind:     firstNonEmpty(strings.TrimSpace(item.Role), "goal_item"),
				Text:     strings.TrimSpace(item.Text),
				Required: true,
			})
		}
	}
	if sources != nil && sources.Intake != nil {
		for i, antiGoal := range sources.Intake.AntiGoals {
			text := strings.TrimSpace(antiGoal)
			if text == "" {
				continue
			}
			model.AntiGoals = append(model.AntiGoals, SuccessAntiGoal{
				ID:   fmt.Sprintf("intake-%d", i+1),
				Text: text,
			})
		}
	}
	return model
}

func compileBootstrapProofPlan(goalState *GoalState, acceptanceState *AcceptanceState, successModel *SuccessModel) *ProofPlan {
	plan := &ProofPlan{
		Version: 1,
		Items: []ProofPlanItem{
			{
				ID:               "proof-summary-objective",
				CoversDimensions: []string{"dim-objective"},
				Kind:             "run_artifact",
				Required:         true,
				SourceSurface:    "summary",
			},
		},
	}
	if goalState != nil {
		for _, item := range goalState.Required {
			if strings.TrimSpace(item.ID) == "" {
				continue
			}
			proofKind := "run_artifact"
			sourceSurface := "summary"
			if strings.TrimSpace(item.Role) == goalItemRoleProof {
				proofKind = "acceptance_check"
				sourceSurface = "acceptance"
			}
			plan.Items = append(plan.Items, ProofPlanItem{
				ID:               "proof-goal-" + goalx.Slugify(item.ID),
				CoversDimensions: []string{item.ID},
				Kind:             proofKind,
				Required:         true,
				SourceSurface:    sourceSurface,
			})
		}
	}
	if acceptanceState != nil {
		for _, check := range acceptanceState.Checks {
			if strings.TrimSpace(check.ID) == "" {
				continue
			}
			plan.Items = append(plan.Items, ProofPlanItem{
				ID:               "proof-acceptance-" + goalx.Slugify(check.ID),
				CoversDimensions: []string{"dim-objective"},
				Kind:             "acceptance_check",
				Required:         true,
				SourceSurface:    "acceptance",
			})
		}
	}
	if successModel != nil && len(successModel.Dimensions) == 1 && len(plan.Items) == 1 {
		plan.Items[0].Kind = "bootstrap_proof"
	}
	return plan
}

func compileBootstrapWorkflowPlan() *WorkflowPlan {
	return &WorkflowPlan{
		Version: 1,
		RequiredRoles: []WorkflowRoleRequirement{
			{ID: "builder", Required: true},
			{ID: "critic", Required: true},
			{ID: "finisher", Required: true},
		},
		Gates: []string{
			"builder_result_present",
			"critic_review_present",
			"finisher_pass_present",
		},
	}
}

func buildBootstrapCompilerSources(projectRoot, runDir string) (*bootstrapCompilerSources, error) {
	query, err := LoadMemoryQueryFile(MemoryQueryPath(runDir))
	if err != nil {
		return nil, err
	}
	context, err := LoadMemoryContextFile(MemoryContextPath(runDir))
	if err != nil {
		return nil, err
	}
	intake, err := LoadLiveRunIntake(runDir)
	if err != nil {
		return nil, err
	}
	sources := &bootstrapCompilerSources{
		Query:   query,
		Context: context,
		Intake:  intake,
	}
	if query != nil {
		selected, rejected, err := evaluateSuccessPriorCandidates(*query)
		if err != nil {
			return nil, err
		}
		for _, entry := range selected {
			sources.PriorEntryIDs = append(sources.PriorEntryIDs, entry.ID)
		}
		sources.RejectedPriors = append(sources.RejectedPriors, rejected...)
	}
	if _, err := os.Stat(filepath.Join(projectRoot, "AGENTS.md")); err == nil {
		sources.PolicySourceRefs = append(sources.PolicySourceRefs, "AGENTS.md")
		sources.SourceSlots = append(sources.SourceSlots, CompilerInputSlot{
			Slot: CompilerInputSlotRepoPolicy,
			Refs: []string{"AGENTS.md"},
		})
	}
	runContextRefs := []string{}
	if context != nil {
		runContextRefs = append(runContextRefs, filepath.Join("control", "memory-context.json"))
	}
	if intake != nil {
		runContextRefs = append(runContextRefs, filepath.Join("control", "intake.json"))
	}
	if len(runContextRefs) > 0 {
		sources.SourceSlots = append(sources.SourceSlots, CompilerInputSlot{
			Slot: CompilerInputSlotRunContext,
			Refs: runContextRefs,
		})
	}
	if len(sources.PriorEntryIDs) > 0 {
		sources.SourceSlots = append(sources.SourceSlots, CompilerInputSlot{
			Slot: CompilerInputSlotLearnedSuccessPriors,
			Refs: append([]string(nil), sources.PriorEntryIDs...),
		})
	}
	return sources, nil
}

func compileBootstrapCompilerInput(runDir string, sources *bootstrapCompilerSources) *CompilerInput {
	input := &CompilerInput{
		Version:              1,
		CompilerVersion:      successCompilerVersion,
		ObjectiveContractRef: filepath.Base(ObjectiveContractPath(runDir)),
		GoalRef:              filepath.Base(GoalPath(runDir)),
	}
	if sources == nil {
		return input
	}
	if sources.Query != nil {
		input.MemoryQueryRef = filepath.Join("control", "memory-query.json")
	}
	if sources.Context != nil {
		input.MemoryContextRef = filepath.Join("control", "memory-context.json")
	}
	input.PolicySourceRefs = append([]string(nil), sources.PolicySourceRefs...)
	input.SelectedPriorRefs = append([]string(nil), sources.PriorEntryIDs...)
	input.SourceSlots = append([]CompilerInputSlot(nil), sources.SourceSlots...)
	return input
}

func compileBootstrapCompilerReport(sources *bootstrapCompilerSources) *CompilerReport {
	report := &CompilerReport{
		Version:         1,
		CompilerVersion: successCompilerVersion,
	}
	if sources == nil {
		return report
	}
	for _, slot := range sources.SourceSlots {
		report.AvailableSourceSlots = append(report.AvailableSourceSlots, CompilerReportSlot{
			Slot: slot.Slot,
			Refs: append([]string(nil), slot.Refs...),
		})
		for _, output := range []string{"success-model", "proof-plan", "workflow-plan", "domain-pack", "protocol-composition"} {
			report.OutputSources = append(report.OutputSources, CompilerOutputSource{
				Output:     output,
				SourceSlot: slot.Slot,
				Refs:       append([]string(nil), slot.Refs...),
			})
		}
	}
	report.SelectedPriorRefs = append([]string(nil), sources.PriorEntryIDs...)
	report.RejectedPriors = append([]CompilerRejectedPrior(nil), sources.RejectedPriors...)
	return report
}

func compileBootstrapDomainPack(cfg *goalx.Config, meta *RunMetadata, sources *bootstrapCompilerSources) (*DomainPack, error) {
	priorEntryIDs := []string{}
	if sources != nil {
		priorEntryIDs = append(priorEntryIDs, sources.PriorEntryIDs...)
	}
	domain := "generic"
	if meta != nil && strings.TrimSpace(meta.Intent) != "" {
		domain = strings.TrimSpace(meta.Intent)
	} else if cfg.Mode != "" {
		domain = strings.ToLower(string(cfg.Mode))
	}
	signals := []string{firstNonEmpty(strings.TrimSpace(string(cfg.Mode)), "mode_unspecified")}
	if meta != nil && strings.TrimSpace(meta.Intent) != "" {
		signals = append(signals, "intent:"+strings.TrimSpace(meta.Intent))
	}
	if sources != nil && sources.Query != nil && strings.TrimSpace(sources.Query.ProjectID) != "" {
		signals = append(signals, "project:"+strings.TrimSpace(sources.Query.ProjectID))
	}
	if sources != nil && sources.Context != nil && (len(sources.Context.Facts)+len(sources.Context.Procedures)+len(sources.Context.Pitfalls)+len(sources.Context.SecretRefs)+len(sources.Context.SuccessPriors)) > 0 {
		signals = append(signals, "memory_context_present")
	}
	if sources != nil && sources.Intake != nil {
		signals = append(signals, "intake_present")
	}
	if len(priorEntryIDs) > 0 {
		signals = append(signals, "success_prior_present")
	}
	pack := &DomainPack{
		Version:       1,
		Domain:        domain,
		Signals:       signals,
		PriorEntryIDs: priorEntryIDs,
	}
	if sources != nil {
		if len(sources.PolicySourceRefs) > 0 {
			pack.Slots.RepoPolicy = DomainPackSlot{
				Source: sources.PolicySourceRefs[0],
				Refs:   append([]string(nil), sources.PolicySourceRefs...),
			}
		}
		if sources.Context != nil {
			pack.Slots.RunContext = DomainPackSlot{
				Source: filepath.Join("control", "memory-context.json"),
				Refs:   []string{filepath.Join("control", "memory-context.json")},
			}
		}
		if len(priorEntryIDs) > 0 {
			pack.Slots.LearnedSuccessPriors = DomainPackSlot{
				EntryIDs: append([]string(nil), priorEntryIDs...),
			}
		}
	}
	return pack, nil
}

func evaluateSuccessPriorCandidates(query MemoryQuery) ([]MemoryEntry, []CompilerRejectedPrior, error) {
	entries, err := RetrieveMemory(query)
	if err != nil {
		return nil, nil, err
	}
	selected := make([]MemoryEntry, 0)
	selectedSet := make(map[string]struct{})
	for _, entry := range entries {
		if entry.Kind != MemoryKindSuccessPrior {
			continue
		}
		selected = append(selected, entry)
		selectedSet[entry.ID] = struct{}{}
	}

	allEntries, err := loadCanonicalEntriesForKind(MemoryKindSuccessPrior)
	if err != nil {
		return nil, nil, err
	}
	governance, err := loadMemoryPriorGovernanceSummary()
	if err != nil {
		return nil, nil, err
	}
	rejected := make([]CompilerRejectedPrior, 0)
	for _, entry := range allEntries {
		if _, ok := selectedSet[entry.ID]; ok {
			continue
		}
		rejected = append(rejected, CompilerRejectedPrior{
			Ref:        entry.ID,
			ReasonCode: compilerRejectReasonForSuccessPrior(entry, query, governance[entry.ID]),
		})
	}
	return selected, rejected, nil
}

func compilerRejectReasonForSuccessPrior(entry MemoryEntry, query MemoryQuery, summary memoryPriorGovernanceSummary) string {
	if strings.TrimSpace(firstNonEmpty(summary.SupersededBy, entry.SupersededBy)) != "" {
		return CompilerReasonSuperseded
	}
	if !memoryEntrySelectorsMatch(entry, query) {
		return CompilerReasonNoSelectorMatch
	}
	if entry.ContradictedCount+summary.ContradictedCount > 0 {
		return CompilerReasonContradicted
	}
	return CompilerReasonLowerPriority
}

func memoryEntrySelectorsMatch(entry MemoryEntry, query MemoryQuery) bool {
	querySelectors := querySelectorMap(query)
	for key, value := range entry.Selectors {
		queryValue := querySelectors[key]
		if queryValue == "" {
			continue
		}
		if queryValue != value {
			return false
		}
	}
	return true
}

func compilerInputSignature(input *CompilerInput) string {
	if input == nil {
		return ""
	}
	payload := struct {
		SelectedPriorRefs []string            `json:"selected_prior_refs,omitempty"`
		SourceSlots       []CompilerInputSlot `json:"source_slots,omitempty"`
	}{
		SelectedPriorRefs: append([]string(nil), input.SelectedPriorRefs...),
		SourceSlots:       append([]CompilerInputSlot(nil), input.SourceSlots...),
	}
	data, _ := json.Marshal(payload)
	return string(data)
}

func hashFileSHA256(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

func successPriorStatements(context *MemoryContext) []string {
	if context == nil {
		return nil
	}
	return append([]string(nil), context.SuccessPriors...)
}

func domainPackPriorIDs(pack *DomainPack) []string {
	if pack == nil {
		return nil
	}
	return append([]string(nil), pack.PriorEntryIDs...)
}
