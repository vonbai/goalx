package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type DomainPack struct {
	Version       int             `json:"version"`
	CompiledAt    string          `json:"compiled_at,omitempty"`
	Domain        string          `json:"domain"`
	Signals       []string        `json:"signals,omitempty"`
	Slots         DomainPackSlots `json:"slots,omitempty"`
	PriorEntryIDs []string        `json:"prior_entry_ids,omitempty"`
}

type DomainPackSlots struct {
	RepoPolicy           DomainPackSlot `json:"repo_policy,omitempty"`
	LearnedSuccessPriors DomainPackSlot `json:"learned_success_priors,omitempty"`
	RunContext           DomainPackSlot `json:"run_context,omitempty"`
}

type DomainPackSlot struct {
	Source   string   `json:"source,omitempty"`
	Refs     []string `json:"refs,omitempty"`
	EntryIDs []string `json:"entry_ids,omitempty"`
}

func DomainPackPath(runDir string) string {
	return filepath.Join(runDir, "domain-pack.json")
}

func LoadDomainPack(path string) (*DomainPack, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	pack, err := parseDomainPack(data)
	if err != nil {
		return nil, fmt.Errorf("parse domain pack: %w", err)
	}
	return pack, nil
}

func SaveDomainPack(path string, pack *DomainPack) error {
	if pack == nil {
		return fmt.Errorf("domain pack is nil")
	}
	if err := validateDomainPackInput(pack); err != nil {
		return err
	}
	normalizeDomainPack(pack)
	if pack.CompiledAt == "" {
		pack.CompiledAt = time.Now().UTC().Format(time.RFC3339)
	}
	data, err := json.MarshalIndent(pack, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(path, data, 0o644)
}

func parseDomainPack(data []byte) (*DomainPack, error) {
	var pack DomainPack
	if len(strings.TrimSpace(string(data))) == 0 {
		return nil, durableSchemaHintError(DurableSurfaceDomainPack, fmt.Errorf("domain pack is empty"))
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&pack); err != nil {
		return nil, durableSchemaHintError(DurableSurfaceDomainPack, err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return nil, durableSchemaHintError(DurableSurfaceDomainPack, err)
	}
	if err := validateDomainPackInput(&pack); err != nil {
		return nil, durableSchemaHintError(DurableSurfaceDomainPack, err)
	}
	normalizeDomainPack(&pack)
	return &pack, nil
}

func validateDomainPackInput(pack *DomainPack) error {
	if pack == nil {
		return fmt.Errorf("domain pack is nil")
	}
	if pack.Version <= 0 {
		return fmt.Errorf("domain pack version must be positive")
	}
	if strings.TrimSpace(pack.Domain) == "" {
		return fmt.Errorf("domain pack domain is required")
	}
	if hasDomainPackSlotData(pack.Slots.RepoPolicy) && strings.TrimSpace(pack.Slots.RepoPolicy.Source) == "" {
		return fmt.Errorf("domain pack repo_policy slot source is required")
	}
	if hasDomainPackSlotData(pack.Slots.RunContext) && strings.TrimSpace(pack.Slots.RunContext.Source) == "" {
		return fmt.Errorf("domain pack run_context slot source is required")
	}
	if len(pack.PriorEntryIDs) > 0 && len(pack.Slots.LearnedSuccessPriors.EntryIDs) > 0 &&
		!stringSliceEqual(compactStrings(pack.PriorEntryIDs), compactStrings(pack.Slots.LearnedSuccessPriors.EntryIDs)) {
		return fmt.Errorf("domain pack prior_entry_ids must match learned_success_priors.entry_ids")
	}
	return nil
}

func normalizeDomainPack(pack *DomainPack) {
	if pack.Version <= 0 {
		pack.Version = 1
	}
	pack.CompiledAt = strings.TrimSpace(pack.CompiledAt)
	pack.Domain = strings.TrimSpace(pack.Domain)
	pack.Signals = compactStrings(pack.Signals)
	normalizeDomainPackSlot(&pack.Slots.RepoPolicy)
	normalizeDomainPackSlot(&pack.Slots.LearnedSuccessPriors)
	normalizeDomainPackSlot(&pack.Slots.RunContext)
	pack.PriorEntryIDs = compactStrings(pack.PriorEntryIDs)
}

func normalizeDomainPackSlot(slot *DomainPackSlot) {
	if slot == nil {
		return
	}
	slot.Source = strings.TrimSpace(slot.Source)
	slot.Refs = compactStrings(slot.Refs)
	slot.EntryIDs = compactStrings(slot.EntryIDs)
}

func hasDomainPackSlotData(slot DomainPackSlot) bool {
	return strings.TrimSpace(slot.Source) != "" || len(slot.Refs) > 0 || len(slot.EntryIDs) > 0
}
