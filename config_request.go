package goalx

// ResolveRequest captures per-run overrides that are applied on top of loaded config layers.
type ResolveRequest struct {
	ManualDraft               *Config
	Name                      string
	Mode                      Mode
	Objective                 string
	Parallel                  int
	ClearSessions             bool
	RequireEngineAvailability bool
	TargetOverride            *TargetConfig
	LocalValidationOverride   *LocalValidationConfig
	MasterOverride            *MasterConfig
	ResearchOverride          *SessionConfig
	DevelopOverride           *SessionConfig
}
