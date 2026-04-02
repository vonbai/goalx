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

// RunCharter records immutable structural facts about a run.
// Strategy and governance (completion standard, approval gates, exploration doctrine)
// live in the master template as semantic guidance, not in Go structs.
type RunCharter struct {
	Version       int                     `json:"version"`
	CharterID     string                  `json:"charter_id,omitempty"`
	RunID         string                  `json:"run_id,omitempty"`
	RootRunID     string                  `json:"root_run_id,omitempty"`
	RunName       string                  `json:"run_name,omitempty"`
	ProjectID     string                  `json:"project_id,omitempty"`
	ProjectRoot   string                  `json:"project_root,omitempty"`
	Objective     string                  `json:"objective,omitempty"`
	Mode          string                  `json:"mode,omitempty"`
	PhaseKind     string                  `json:"phase_kind,omitempty"`
	SourceRun     string                  `json:"source_run,omitempty"`
	SourcePhase   string                  `json:"source_phase,omitempty"`
	ParentRun     string                  `json:"parent_run,omitempty"`
	RoleContracts RunCharterRoleContracts `json:"role_contracts,omitempty"`
	Paths         RunCharterPaths         `json:"paths,omitempty"`
	CreatedAt     string                  `json:"created_at,omitempty"`
}

type RunCharterPaths struct {
	Charter         string `json:"charter,omitempty"`
	ObligationModel string `json:"obligation_model,omitempty"`
	AssurancePlan   string `json:"assurance_plan,omitempty"`
	Proof           string `json:"proof,omitempty"`
}

type RunCharterRoleContracts struct {
	Master *RoleContract `json:"master,omitempty"`
	Worker *RoleContract `json:"worker,omitempty"`
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

func NewRunCharter(runDir, runName, objective string, meta *RunMetadata) (*RunCharter, error) {
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
		Version:     1,
		CharterID:   meta.CharterID,
		RunID:       runID,
		RootRunID:   rootRunID,
		RunName:     runName,
		ProjectRoot: meta.ProjectRoot,
		Objective:   strings.TrimSpace(objective),
		Mode:        "",
		PhaseKind:   meta.PhaseKind,
		SourceRun:   meta.SourceRun,
		SourcePhase: meta.SourcePhase,
		ParentRun:   meta.ParentRun,
		RoleContracts: RunCharterRoleContracts{
			Master: &RoleContract{
				Kind:    "master",
				Mandate: "Drive the run, compare paths, and optimize for the user objective.",
			},
			Worker: &RoleContract{
				Kind:    "worker",
				Mandate: "Execute assigned slices, produce durable evidence, and return code or reports the master can compare and integrate.",
			},
		},
		Paths: RunCharterPaths{
			Charter:         RunCharterPath(runDir),
			ObligationModel: ObligationModelPath(runDir),
			AssurancePlan:   AssurancePlanPath(runDir),
			Proof:           CompletionStatePath(runDir),
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
	type runCharterCompat struct {
		Version       int                     `json:"version"`
		CharterID     string                  `json:"charter_id,omitempty"`
		RunID         string                  `json:"run_id,omitempty"`
		RootRunID     string                  `json:"root_run_id,omitempty"`
		RunName       string                  `json:"run_name,omitempty"`
		ProjectID     string                  `json:"project_id,omitempty"`
		ProjectRoot   string                  `json:"project_root,omitempty"`
		Objective     string                  `json:"objective,omitempty"`
		Mode          string                  `json:"mode,omitempty"`
		PhaseKind     string                  `json:"phase_kind,omitempty"`
		SourceRun     string                  `json:"source_run,omitempty"`
		SourcePhase   string                  `json:"source_phase,omitempty"`
		ParentRun     string                  `json:"parent_run,omitempty"`
		RoleContracts RunCharterRoleContracts `json:"role_contracts,omitempty"`
		Paths         struct {
			Charter         string `json:"charter,omitempty"`
			ObligationModel string `json:"obligation_model,omitempty"`
			AssurancePlan   string `json:"assurance_plan,omitempty"`
			Goal            string `json:"goal,omitempty"`
			Acceptance      string `json:"acceptance,omitempty"`
			Proof           string `json:"proof,omitempty"`
		} `json:"paths,omitempty"`
		CreatedAt string `json:"created_at,omitempty"`
	}
	var payload runCharterCompat
	if len(strings.TrimSpace(string(data))) == 0 {
		charter := &RunCharter{Version: 1}
		normalizeRunCharter(charter)
		return charter, nil
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("parse run charter: %w", err)
	}
	charter := RunCharter{
		Version:       payload.Version,
		CharterID:     payload.CharterID,
		RunID:         payload.RunID,
		RootRunID:     payload.RootRunID,
		RunName:       payload.RunName,
		ProjectID:     payload.ProjectID,
		ProjectRoot:   payload.ProjectRoot,
		Objective:     payload.Objective,
		Mode:          payload.Mode,
		PhaseKind:     payload.PhaseKind,
		SourceRun:     payload.SourceRun,
		SourcePhase:   payload.SourcePhase,
		ParentRun:     payload.ParentRun,
		RoleContracts: payload.RoleContracts,
		Paths: RunCharterPaths{
			Charter:         payload.Paths.Charter,
			ObligationModel: firstNonEmpty(strings.TrimSpace(payload.Paths.ObligationModel), strings.TrimSpace(payload.Paths.Goal)),
			AssurancePlan:   firstNonEmpty(strings.TrimSpace(payload.Paths.AssurancePlan), strings.TrimSpace(payload.Paths.Acceptance)),
			Proof:           payload.Paths.Proof,
		},
		CreatedAt: payload.CreatedAt,
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
	if charter.RoleContracts.Master == nil {
		charter.RoleContracts.Master = &RoleContract{
			Kind:    "master",
			Mandate: "Drive the run, compare paths, and optimize for the user objective.",
		}
	}
	if charter.RoleContracts.Worker == nil {
		charter.RoleContracts.Worker = &RoleContract{
			Kind:    "worker",
			Mandate: "Execute assigned slices, produce durable evidence, and return code or reports the master can compare and integrate.",
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
