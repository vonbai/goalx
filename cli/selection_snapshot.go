package cli

import (
	"fmt"
	"os"
	"path/filepath"

	goalx "github.com/vonbai/goalx"
)

type SelectionSnapshot struct {
	Version           int                            `json:"version"`
	ExplicitSelection bool                           `json:"explicit_selection,omitempty"`
	Policy            goalx.EffectiveSelectionPolicy `json:"policy"`
	Master            goalx.MasterConfig             `json:"master"`
	Research          goalx.SessionConfig            `json:"research"`
	Develop           goalx.SessionConfig            `json:"develop"`
}

func SelectionSnapshotPath(runDir string) string {
	return filepath.Join(runDir, "selection-policy.json")
}

func BuildSelectionSnapshot(cfg *goalx.Config, policy goalx.EffectiveSelectionPolicy, explicit bool) *SelectionSnapshot {
	if cfg == nil {
		return nil
	}
	if selectionPolicyEmpty(policy) {
		policy = goalx.DeriveSelectionPolicy(cfg)
	}
	return &SelectionSnapshot{
		Version:           1,
		ExplicitSelection: explicit,
		Policy:            copySelectionPolicy(policy),
		Master:            cfg.Master,
		Research:          cfg.Roles.Research,
		Develop:           cfg.Roles.Develop,
	}
}

func LoadSelectionSnapshot(path string) (*SelectionSnapshot, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	snapshot := &SelectionSnapshot{}
	if err := decodeStrictJSON(data, snapshot); err != nil {
		return nil, fmt.Errorf("parse selection snapshot: %w", err)
	}
	if snapshot.Version == 0 {
		snapshot.Version = 1
	}
	return snapshot, nil
}

func SaveSelectionSnapshot(runDir string, snapshot *SelectionSnapshot) error {
	if snapshot == nil {
		return fmt.Errorf("selection snapshot is nil")
	}
	if snapshot.Version == 0 {
		snapshot.Version = 1
	}
	return writeJSONFile(SelectionSnapshotPath(runDir), snapshot)
}

func applySelectionSnapshotConfig(cfg *goalx.Config, snapshot *SelectionSnapshot) {
	if cfg == nil || snapshot == nil {
		return
	}
	cfg.Master = snapshot.Master
	cfg.Roles.Research = snapshot.Research
	cfg.Roles.Develop = snapshot.Develop
}

func selectionPolicyEmpty(policy goalx.EffectiveSelectionPolicy) bool {
	return len(policy.DisabledEngines) == 0 &&
		len(policy.DisabledTargets) == 0 &&
		len(policy.MasterCandidates) == 0 &&
		len(policy.ResearchCandidates) == 0 &&
		len(policy.DevelopCandidates) == 0 &&
		policy.MasterEffort == "" &&
		policy.ResearchEffort == "" &&
		policy.DevelopEffort == ""
}

func copySelectionPolicy(policy goalx.EffectiveSelectionPolicy) goalx.EffectiveSelectionPolicy {
	return goalx.EffectiveSelectionPolicy{
		DisabledEngines:    append([]string(nil), policy.DisabledEngines...),
		DisabledTargets:    append([]string(nil), policy.DisabledTargets...),
		MasterCandidates:   append([]string(nil), policy.MasterCandidates...),
		ResearchCandidates: append([]string(nil), policy.ResearchCandidates...),
		DevelopCandidates:  append([]string(nil), policy.DevelopCandidates...),
		MasterEffort:       policy.MasterEffort,
		ResearchEffort:     policy.ResearchEffort,
		DevelopEffort:      policy.DevelopEffort,
	}
}
