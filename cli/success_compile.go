package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	goalx "github.com/vonbai/goalx"
)

const successCompilerVersion = "bootstrap-v1"

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

	successModel := compileBootstrapSuccessModel(cfg, objectiveContract, goalState, objectiveContractHash, goalHash)
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
	domainPack, err := compileBootstrapDomainPack(projectRoot, runDir, cfg, meta)
	if err != nil {
		return err
	}
	if err := SaveDomainPack(DomainPackPath(runDir), domainPack); err != nil {
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
		domainPack, err := compileBootstrapDomainPack(projectRoot, runDir, cfg, meta)
		if err != nil {
			return false, err
		}
		if err := SaveDomainPack(DomainPackPath(runDir), domainPack); err != nil {
			return false, err
		}
	}

	afterContext, err := LoadMemoryContextFile(MemoryContextPath(runDir))
	if err != nil {
		return false, err
	}
	afterDomainPack, err := LoadDomainPack(DomainPackPath(runDir))
	if err != nil {
		return false, err
	}
	return !stringSliceEqual(successPriorStatements(beforeContext), successPriorStatements(afterContext)) ||
		!stringSliceEqual(domainPackPriorIDs(beforeDomainPack), domainPackPriorIDs(afterDomainPack)), nil
}

func compileBootstrapSuccessModel(cfg *goalx.Config, objectiveContract *ObjectiveContract, goalState *GoalState, objectiveContractHash, goalHash string) *SuccessModel {
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

func compileBootstrapDomainPack(projectRoot, runDir string, cfg *goalx.Config, meta *RunMetadata) (*DomainPack, error) {
	query, err := LoadMemoryQueryFile(MemoryQueryPath(runDir))
	if err != nil {
		return nil, err
	}
	context, err := LoadMemoryContextFile(MemoryContextPath(runDir))
	if err != nil {
		return nil, err
	}
	priorEntryIDs := []string{}
	if query != nil {
		entries, err := RetrieveMemory(*query)
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			if entry.Kind != MemoryKindSuccessPrior {
				continue
			}
			priorEntryIDs = append(priorEntryIDs, entry.ID)
		}
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
	if query != nil && strings.TrimSpace(query.ProjectID) != "" {
		signals = append(signals, "project:"+strings.TrimSpace(query.ProjectID))
	}
	if context != nil && (len(context.Facts)+len(context.Procedures)+len(context.Pitfalls)+len(context.SecretRefs)+len(context.SuccessPriors)) > 0 {
		signals = append(signals, "memory_context_present")
	}
	if len(priorEntryIDs) > 0 {
		signals = append(signals, "success_prior_present")
	}
	policySources := []string{}
	if _, err := os.Stat(filepath.Join(projectRoot, "AGENTS.md")); err == nil {
		policySources = append(policySources, "AGENTS.md")
	}
	return &DomainPack{
		Version:       1,
		Domain:        domain,
		Signals:       signals,
		PolicySources: policySources,
		PriorEntryIDs: priorEntryIDs,
	}, nil
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
