package cli

import "fmt"

type DurableSurfaceClass string
type DurableSurfaceWriteMode string
type DurableSurfaceName string

const (
	DurableSurfaceClassStructuredState DurableSurfaceClass = "structured_state"
	DurableSurfaceClassEventLog        DurableSurfaceClass = "event_log"
	DurableSurfaceClassArtifact        DurableSurfaceClass = "artifact"

	DurableSurfaceWriteModeReplace DurableSurfaceWriteMode = "replace"
	DurableSurfaceWriteModeAppend  DurableSurfaceWriteMode = "append"

	DurableSurfaceGoal            DurableSurfaceName = "goal"
	DurableSurfaceAcceptance      DurableSurfaceName = "acceptance"
	DurableSurfaceCoordination    DurableSurfaceName = "coordination"
	DurableSurfaceStatus          DurableSurfaceName = "status"
	DurableSurfaceGoalLog         DurableSurfaceName = "goal-log"
	DurableSurfaceExperiments     DurableSurfaceName = "experiments"
	DurableSurfaceSummary         DurableSurfaceName = "summary"
	DurableSurfaceCompletionProof DurableSurfaceName = "completion-proof"
)

type DurableSurfaceSpec struct {
	Name               DurableSurfaceName
	Class              DurableSurfaceClass
	WriteMode          DurableSurfaceWriteMode
	Strict             bool
	FrameworkReadsBody bool
	Schema             DurableSurfaceSchemaSpec
	Path               func(runDir string) string
}

var durableSurfaceRegistry = map[DurableSurfaceName]DurableSurfaceSpec{
	DurableSurfaceGoal: {
		Name:               DurableSurfaceGoal,
		Class:              DurableSurfaceClassStructuredState,
		WriteMode:          DurableSurfaceWriteModeReplace,
		Strict:             true,
		FrameworkReadsBody: true,
		Schema: DurableSurfaceSchemaSpec{
			Format:  DurableSurfaceSchemaFormatJSON,
			Summary: "Current mutable goal boundary with required outcomes and optional improvements.",
			Example: `{"version":1,"required":[{"id":"req-1","text":"ship feature","source":"user","state":"open"}],"optional":[{"id":"opt-1","text":"improve latency","source":"master","state":"open"}],"updated_at":"2026-03-28T10:00:00Z"}`,
			FieldNotes: []string{
				"`required` is the canonical current-goal boundary.",
				"`state` must stay within open|claimed|waived.",
				"`waived` only counts with explicit user approval on the item.",
			},
		},
		Path: GoalPath,
	},
	DurableSurfaceAcceptance: {
		Name:               DurableSurfaceAcceptance,
		Class:              DurableSurfaceClassStructuredState,
		WriteMode:          DurableSurfaceWriteModeReplace,
		Strict:             true,
		FrameworkReadsBody: true,
		Schema: DurableSurfaceSchemaSpec{
			Format:  DurableSurfaceSchemaFormatJSON,
			Summary: "Verification command surface and latest raw acceptance result.",
			Example: `{"version":1,"goal_version":1,"default_command":"go test ./...","effective_command":"go test ./...","last_result":{"checked_at":"2026-03-28T10:00:00Z","command":"go test ./...","exit_code":0,"evidence_path":"/abs/run/acceptance-last.txt"},"updated_at":"2026-03-28T10:00:00Z"}`,
			FieldNotes: []string{
				"`default_command` is the launch baseline command.",
				"`effective_command` is the current command used by verify.",
				"`last_result` records facts only; no completion judgment fields.",
			},
		},
		Path: AcceptanceStatePath,
	},
	DurableSurfaceCoordination: {
		Name:               DurableSurfaceCoordination,
		Class:              DurableSurfaceClassStructuredState,
		WriteMode:          DurableSurfaceWriteModeReplace,
		Strict:             true,
		FrameworkReadsBody: true,
		Schema: DurableSurfaceSchemaSpec{
			Format:  DurableSurfaceSchemaFormatJSON,
			Summary: "Short master-written coordination digest for ownership and execution state.",
			Example: `{"version":1,"plan_summary":["session-1 explores root cause"],"owners":{"req-1":"session-1"},"sessions":{"session-1":{"state":"active","execution_state":"working","scope":"trace unknown field source","last_round":1,"updated_at":"2026-03-28T10:00:00Z"}},"decision":{"root_cause":"legacy schema drift","chosen_path":"single_source_contract","chosen_path_reason":"one concern one path"},"blocked":[],"open_questions":[],"updated_at":"2026-03-28T10:00:00Z"}`,
			FieldNotes: []string{
				"No legacy aliases are accepted for session grouping fields.",
				"`owners` is explicit coverage mapping when present.",
				"Keep verbose reasoning in journals, not this digest.",
			},
		},
		Path: CoordinationPath,
	},
	DurableSurfaceStatus: {
		Name:               DurableSurfaceStatus,
		Class:              DurableSurfaceClassStructuredState,
		WriteMode:          DurableSurfaceWriteModeReplace,
		Strict:             true,
		FrameworkReadsBody: true,
		Schema: DurableSurfaceSchemaSpec{
			Format:  DurableSurfaceSchemaFormatJSON,
			Summary: "Master-written run progress facts for phase, remaining required work, and ownership.",
			Example: `{"version":1,"phase":"working","required_remaining":0,"open_required_ids":["req-1"],"active_sessions":["session-1"],"keep_session":"session-2","last_verified_at":"2026-03-28T10:00:00Z","updated_at":"2026-03-28T10:05:00Z"}`,
			FieldNotes: []string{
				"`required_remaining` is required and must be non-negative.",
				"`phase` is restricted to working|review|complete.",
				"No recommendation or completion verdict fields are accepted.",
			},
		},
		Path: RunStatusPath,
	},
	DurableSurfaceGoalLog: {
		Name:               DurableSurfaceGoalLog,
		Class:              DurableSurfaceClassEventLog,
		WriteMode:          DurableSurfaceWriteModeAppend,
		Strict:             true,
		FrameworkReadsBody: false,
		Schema: DurableSurfaceSchemaSpec{
			Format:  DurableSurfaceSchemaFormatJSONL,
			Summary: "Append-only goal boundary and coverage change events using canonical durable-log envelope.",
			Example: `{"version":1,"kind":"update","at":"2026-03-28T10:00:00Z","actor":"master","body":{"goal_version":2,"reason":"tighten required outcomes"}}`,
			FieldNotes: []string{
				"Each line must be one valid JSON object.",
				"Envelope fields are required: version, kind, at, actor, body.",
				"`body` must be a JSON object.",
			},
		},
		Path: GoalLogPath,
	},
	DurableSurfaceExperiments: {
		Name:               DurableSurfaceExperiments,
		Class:              DurableSurfaceClassEventLog,
		WriteMode:          DurableSurfaceWriteModeAppend,
		Strict:             true,
		FrameworkReadsBody: false,
		Schema: DurableSurfaceSchemaSpec{
			Format:  DurableSurfaceSchemaFormatJSONL,
			Summary: "Append-only experiment lineage events using canonical durable-log envelope.",
			Example: `{"version":1,"kind":"experiment.created","at":"2026-03-28T10:00:00Z","actor":"session-1","body":{"experiment_id":"exp-1","created_at":"2026-03-28T10:00:00Z"}}`,
			FieldNotes: []string{
				"Each line must be one valid JSON object.",
				"Use only allowed `kind` values for the experiments surface.",
				"Do not infer verdicts from `body`; interpretation belongs to agents.",
			},
		},
		Path: ExperimentsLogPath,
	},
	DurableSurfaceSummary: {
		Name:               DurableSurfaceSummary,
		Class:              DurableSurfaceClassArtifact,
		WriteMode:          DurableSurfaceWriteModeReplace,
		Strict:             false,
		FrameworkReadsBody: false,
		Schema: DurableSurfaceSchemaSpec{
			Format:  DurableSurfaceSchemaFormatMarkdown,
			Summary: "Opaque closeout summary artifact for user-facing result narrative.",
			Example: "# Summary\n\n- Outcome\n- Evidence links\n- Next steps\n",
			FieldNotes: []string{
				"Framework treats this as an opaque artifact.",
				"No strict schema validation is applied.",
			},
		},
		Path: SummaryPath,
	},
	DurableSurfaceCompletionProof: {
		Name:               DurableSurfaceCompletionProof,
		Class:              DurableSurfaceClassArtifact,
		WriteMode:          DurableSurfaceWriteModeReplace,
		Strict:             false,
		FrameworkReadsBody: false,
		Schema: DurableSurfaceSchemaSpec{
			Format:  DurableSurfaceSchemaFormatJSON,
			Summary: "Opaque completion-proof artifact authored by the master.",
			Example: `{"notes":"master-owned completion interpretation artifact"}`,
			FieldNotes: []string{
				"Framework treats this as an opaque artifact surface.",
				"No canonical semantic fields are enforced here.",
			},
		},
		Path: CompletionStatePath,
	},
}

func LookupDurableSurface(name string) (DurableSurfaceSpec, error) {
	spec, ok := durableSurfaceRegistry[DurableSurfaceName(name)]
	if !ok {
		return DurableSurfaceSpec{}, fmt.Errorf("unknown durable surface %q", name)
	}
	return spec, nil
}
