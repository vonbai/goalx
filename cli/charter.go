package cli

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	goalx "github.com/vonbai/goalx"
)

type RunCharter struct {
	Version                               int                     `json:"version"`
	CharterID                             string                  `json:"charter_id,omitempty"`
	RunID                                 string                  `json:"run_id,omitempty"`
	RootRunID                             string                  `json:"root_run_id,omitempty"`
	RunName                               string                  `json:"run_name,omitempty"`
	ProjectID                             string                  `json:"project_id,omitempty"`
	ProjectRoot                           string                  `json:"project_root,omitempty"`
	Objective                             string                  `json:"objective,omitempty"`
	Mode                                  string                  `json:"mode,omitempty"`
	PhaseKind                             string                  `json:"phase_kind,omitempty"`
	SourceRun                             string                  `json:"source_run,omitempty"`
	SourcePhase                           string                  `json:"source_phase,omitempty"`
	ParentRun                             string                  `json:"parent_run,omitempty"`
	CompletionStandard                    string                  `json:"completion_standard,omitempty"`
	PartialCompletionRequiresUserApproval bool                    `json:"partial_completion_requires_user_approval,omitempty"`
	NarrowScopeRequiresUserApproval       bool                    `json:"narrow_scope_requires_user_approval,omitempty"`
	RequiredOutcomesMayExpandButNotShrink bool                    `json:"required_outcomes_may_expand_but_not_shrink,omitempty"`
	AcceptanceIsVerificationOnly          bool                    `json:"acceptance_is_verification_only,omitempty"`
	ExplorationDoctrine                   ExplorationDoctrine     `json:"exploration_doctrine,omitempty"`
	RoleContracts                         RunCharterRoleContracts `json:"role_contracts,omitempty"`
	Paths                                 RunCharterPaths         `json:"paths,omitempty"`
	CreatedAt                             string                  `json:"created_at,omitempty"`
}

type RunCharterPaths struct {
	Charter    string `json:"charter,omitempty"`
	Goal       string `json:"goal,omitempty"`
	Acceptance string `json:"acceptance,omitempty"`
	Proof      string `json:"proof,omitempty"`
}

type ExplorationDoctrine struct {
	MinimumPaths              int  `json:"minimum_paths,omitempty"`
	ComparePathsBeforeCommit  bool `json:"compare_paths_before_commit,omitempty"`
	AllowAutonomousPathSwitch bool `json:"allow_autonomous_path_switch,omitempty"`
}

type RunCharterRoleContracts struct {
	Master           *RoleContract `json:"master,omitempty"`
	ResearchSubagent *RoleContract `json:"research_subagent,omitempty"`
	DevelopSubagent  *RoleContract `json:"develop_subagent,omitempty"`
}

type RoleContract struct {
	Kind    string `json:"kind,omitempty"`
	Mandate string `json:"mandate,omitempty"`
}

func RunCharterPath(runDir string) string {
	return filepath.Join(runDir, "run-charter.json")
}

func RequireRunCharter(runDir string) (*RunCharter, error) {
	charter, err := LoadRunCharter(RunCharterPath(runDir))
	if err != nil {
		return nil, err
	}
	if charter == nil {
		return nil, fmt.Errorf("run charter missing at %s", RunCharterPath(runDir))
	}
	return charter, nil
}

func NewRunCharter(runDir, runName string, meta *RunMetadata) (*RunCharter, error) {
	if meta == nil {
		return nil, fmt.Errorf("run metadata is nil")
	}
	runID := strings.TrimSpace(meta.RunID)
	if runID == "" {
		runID = newRunID()
	}
	rootRunID := strings.TrimSpace(meta.RootRunID)
	if rootRunID == "" {
		rootRunID = runID
	}
	charter := &RunCharter{
		Version:                               1,
		CharterID:                             meta.CharterID,
		RunID:                                 runID,
		RootRunID:                             rootRunID,
		RunName:                               runName,
		ProjectRoot:                           meta.ProjectRoot,
		Objective:                             meta.Objective,
		Mode:                                  "",
		PhaseKind:                             meta.PhaseKind,
		SourceRun:                             meta.SourceRun,
		SourcePhase:                           meta.SourcePhase,
		ParentRun:                             meta.ParentRun,
		CompletionStandard:                    "full_goal",
		PartialCompletionRequiresUserApproval: true,
		NarrowScopeRequiresUserApproval:       true,
		RequiredOutcomesMayExpandButNotShrink: true,
		AcceptanceIsVerificationOnly:          true,
		ExplorationDoctrine: ExplorationDoctrine{
			MinimumPaths:              3,
			ComparePathsBeforeCommit:  true,
			AllowAutonomousPathSwitch: true,
		},
		RoleContracts: RunCharterRoleContracts{
			Master: &RoleContract{
				Kind:    "master",
				Mandate: "Drive the run, compare paths, and optimize for the user objective.",
			},
			ResearchSubagent: &RoleContract{
				Kind:    "research",
				Mandate: "Explore the problem space and return evidence-backed findings.",
			},
			DevelopSubagent: &RoleContract{
				Kind:    "develop",
				Mandate: "Implement the best path with durable state and tight feedback loops.",
			},
		},
		Paths: RunCharterPaths{
			Charter:    RunCharterPath(runDir),
			Goal:       GoalPath(runDir),
			Acceptance: AcceptanceStatePath(runDir),
			Proof:      CompletionStatePath(runDir),
		},
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if strings.TrimSpace(charter.ProjectRoot) != "" {
		charter.ProjectID = goalx.ProjectID(charter.ProjectRoot)
	}
	if cfg, err := LoadRunSpec(runDir); err == nil && cfg != nil {
		charter.Mode = string(cfg.Mode)
		if charter.Objective == "" {
			charter.Objective = cfg.Objective
		}
	}
	if strings.TrimSpace(charter.CharterID) == "" {
		charter.CharterID = newCharterID()
	}
	normalizeRunCharter(charter)
	return charter, nil
}

func LoadRunCharter(path string) (*RunCharter, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var charter RunCharter
	if len(strings.TrimSpace(string(data))) == 0 {
		charter.Version = 1
		normalizeRunCharter(&charter)
		return &charter, nil
	}
	if err := json.Unmarshal(data, &charter); err != nil {
		return nil, fmt.Errorf("parse run charter: %w", err)
	}
	normalizeRunCharter(&charter)
	return &charter, nil
}

func SaveRunCharter(path string, charter *RunCharter) error {
	if charter == nil {
		return fmt.Errorf("run charter is nil")
	}
	normalizeRunCharter(charter)
	if strings.TrimSpace(charter.CharterID) == "" {
		charter.CharterID = newCharterID()
	}
	if charter.CreatedAt == "" {
		charter.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(charter, "", "  ")
	if err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		if os.IsExist(err) {
			return fmt.Errorf("run charter already exists at %s", path)
		}
		return err
	}
	defer file.Close()
	_, err = file.Write(data)
	return err
}

func hashRunCharter(charter *RunCharter) (string, error) {
	if charter == nil {
		return "", fmt.Errorf("run charter is nil")
	}
	data, err := json.MarshalIndent(charter, "", "  ")
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func ValidateRunCharterLinkage(meta *RunMetadata, charter *RunCharter) error {
	if charter == nil {
		return fmt.Errorf("run charter is missing")
	}
	if strings.TrimSpace(charter.CharterID) == "" {
		return fmt.Errorf("run charter is missing charter_id")
	}
	if meta == nil {
		return nil
	}
	if meta.CharterID != "" && meta.CharterID != charter.CharterID {
		return fmt.Errorf("run metadata charter_id %q does not match charter %q", meta.CharterID, charter.CharterID)
	}
	if meta.CharterHash != "" {
		want, err := hashRunCharter(charter)
		if err != nil {
			return err
		}
		if meta.CharterHash != want {
			return fmt.Errorf("run metadata charter_hash %q does not match charter hash %q", meta.CharterHash, want)
		}
	}
	for _, check := range []struct {
		name string
		have string
		want string
	}{
		{name: "run_id", have: meta.RunID, want: charter.RunID},
		{name: "root_run_id", have: meta.RootRunID, want: charter.RootRunID},
		{name: "project_root", have: meta.ProjectRoot, want: charter.ProjectRoot},
		{name: "objective", have: meta.Objective, want: charter.Objective},
		{name: "source_run", have: meta.SourceRun, want: charter.SourceRun},
		{name: "source_phase", have: meta.SourcePhase, want: charter.SourcePhase},
		{name: "parent_run", have: meta.ParentRun, want: charter.ParentRun},
	} {
		if strings.TrimSpace(check.have) != "" && check.have != check.want {
			return fmt.Errorf("run metadata %s %q does not match charter %q", check.name, check.have, check.want)
		}
	}
	return nil
}

func ValidateSessionIdentityLinkage(identity *SessionIdentity, charter *RunCharter) error {
	if identity == nil {
		return fmt.Errorf("session identity is nil")
	}
	if charter == nil {
		return fmt.Errorf("run charter is missing")
	}
	if strings.TrimSpace(identity.OriginCharterID) == "" {
		return fmt.Errorf("session identity is missing origin charter id")
	}
	if identity.OriginCharterID != charter.CharterID {
		return fmt.Errorf("session identity charter linkage %q does not match charter %q", identity.OriginCharterID, charter.CharterID)
	}
	return nil
}

func normalizeRunCharter(charter *RunCharter) {
	if charter == nil {
		return
	}
	if charter.Version <= 0 {
		charter.Version = 1
	}
	if charter.CompletionStandard == "" {
		charter.CompletionStandard = "full_goal"
	}
	if charter.ExplorationDoctrine.MinimumPaths <= 0 {
		charter.ExplorationDoctrine.MinimumPaths = 3
	}
	if charter.RoleContracts.Master == nil {
		charter.RoleContracts.Master = &RoleContract{
			Kind:    "master",
			Mandate: "Drive the run, compare paths, and optimize for the user objective.",
		}
	}
	if charter.RoleContracts.ResearchSubagent == nil {
		charter.RoleContracts.ResearchSubagent = &RoleContract{
			Kind:    "research",
			Mandate: "Explore the problem space and return evidence-backed findings.",
		}
	}
	if charter.RoleContracts.DevelopSubagent == nil {
		charter.RoleContracts.DevelopSubagent = &RoleContract{
			Kind:    "develop",
			Mandate: "Implement the best path with durable state and tight feedback loops.",
		}
	}
}

func newCharterID() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("charter_%d", time.Now().UTC().UnixNano())
	}
	return fmt.Sprintf("charter_%x", buf)
}
