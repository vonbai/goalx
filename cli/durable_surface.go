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

	DurableSurfaceObjectiveContract DurableSurfaceName = "objective-contract"
	DurableSurfaceGoal              DurableSurfaceName = "goal"
	DurableSurfaceAcceptance        DurableSurfaceName = "acceptance"
	DurableSurfaceCoordination      DurableSurfaceName = "coordination"
	DurableSurfaceStatus            DurableSurfaceName = "status"
	DurableSurfaceGoalLog           DurableSurfaceName = "goal-log"
	DurableSurfaceExperiments       DurableSurfaceName = "experiments"
	DurableSurfaceSummary           DurableSurfaceName = "summary"
	DurableSurfaceCompletionProof   DurableSurfaceName = "completion-proof"
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
	DurableSurfaceObjectiveContract: {
		Name:               DurableSurfaceObjectiveContract,
		Class:              DurableSurfaceClassStructuredState,
		WriteMode:          DurableSurfaceWriteModeReplace,
		Strict:             true,
		FrameworkReadsBody: true,
		Schema: DurableSurfaceSchemaSpec{
			Format:  DurableSurfaceSchemaFormatJSON,
			Summary: "Immutable extracted user-clause contract for this run.",
			Example: `{"version":1,"objective_hash":"sha256:demo","state":"locked","created_at":"2026-03-29T10:00:00Z","locked_at":"2026-03-29T10:05:00Z","clauses":[{"id":"ucl-1","text":"Live trading works end to end on the live service.","kind":"delivery","source_excerpt":"所有功能端到端真实可用","required_surfaces":["goal"],"approval_required_for_drop":true},{"id":"ucl-2","text":"Playwright user journey passes on the live service.","kind":"verification","source_excerpt":"Playwright 用户旅程测试全部通过（真实服务）","required_surfaces":["goal","acceptance"],"approval_required_for_drop":true}]}`,
			FieldNotes: []string{
				"`objective-contract` is immutable once `state` becomes `locked`.",
				"Each clause must keep a stable `id`, `text`, and `source_excerpt`.",
				"`kind` must stay within delivery|quality_bar|verification|guardrail|operating_rule.",
				"`required_surfaces` must stay within goal|acceptance.",
				"The framework enforces coverage integrity, not semantic satisfaction.",
			},
		},
		Path: ObjectiveContractPath,
	},
	DurableSurfaceGoal: {
		Name:               DurableSurfaceGoal,
		Class:              DurableSurfaceClassStructuredState,
		WriteMode:          DurableSurfaceWriteModeReplace,
		Strict:             true,
		FrameworkReadsBody: true,
		Schema: DurableSurfaceSchemaSpec{
			Format:  DurableSurfaceSchemaFormatJSON,
			Summary: "Current mutable goal boundary with required outcomes and optional improvements.",
			Example: `{"version":1,"required":[{"id":"req-1","text":"Live trading works end to end on the live service with operator-visible state transitions.","source":"user","role":"outcome","state":"open"},{"id":"req-2","text":"Live trading has durable API and browser evidence on the live service.","source":"master","role":"proof","state":"open"}],"optional":[{"id":"opt-1","text":"Improve latency on the live trading dashboard.","source":"master","role":"guardrail","state":"open"}],"updated_at":"2026-03-28T10:00:00Z"}`,
			FieldNotes: []string{
				"`required` is the canonical current-goal boundary.",
				"Every goal item must include explicit `source` and `role` fields.",
				"`role` must stay within outcome|enabler|proof|guardrail.",
				"Describe what must be true, not just how proof will be gathered.",
				"`proof` obligations do not replace missing `outcome` or `enabler` obligations.",
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
			Summary: "Verification check surface and latest raw acceptance results.",
			Example: `{"version":2,"goal_version":1,"checks":[{"id":"chk-build","label":"Go build and test","command":"go build ./... && go test ./... && go vet ./...","covers":["ucl-guardrail"],"state":"active"},{"id":"chk-playwright","label":"Live service user journey","command":"pnpm exec playwright test","covers":["ucl-verify"],"state":"active"}],"last_result":{"checked_at":"2026-03-28T10:00:00Z","exit_code":0,"evidence_path":"/abs/run/acceptance-last.txt","check_results":[{"id":"chk-build","command":"go build ./... && go test ./... && go vet ./...","exit_code":0,"evidence_path":"/abs/run/acceptance-chk-build.txt"}]},"updated_at":"2026-03-28T10:00:00Z"}`,
			FieldNotes: []string{
				"`checks` is the current verification contract for `goalx verify`.",
				"Each check must have stable `id` and `state` fields.",
				"`state` must stay within active|waived.",
				"`waived` checks require explicit `approval_ref`.",
				"`last_result` records raw results only; no completion judgment fields.",
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
			Summary: "Short master-written coordination digest for required-item frontier state and session scope.",
			Example: `{"version":1,"plan_summary":["session-1 explores root cause"],"required":{"req-1":{"owner":"session-1","execution_state":"probing","surfaces":{"repo":"active","runtime":"pending","run_artifacts":"pending","web_research":"pending","external_system":"not_applicable"},"updated_at":"2026-03-28T10:00:00Z"}},"sessions":{"session-1":{"state":"active","scope":"trace unknown field source","last_round":1,"updated_at":"2026-03-28T10:00:00Z"}},"decision":{"root_cause":"legacy schema drift","chosen_path":"single_source_contract","chosen_path_reason":"one concern one path"},"open_questions":[],"updated_at":"2026-03-28T10:00:00Z"}`,
			FieldNotes: []string{
				"No legacy aliases are accepted for session grouping fields.",
				"`required` is the canonical required-item frontier map.",
				"Coverage derives `premature_blocked` when a `blocked` item still has non-terminal machine surfaces.",
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
			Example: `{"version":1,"kind":"decision","at":"2026-03-28T10:00:00Z","actor":"master","body":{"goal_version":2,"decision":"initial_boundary_shape_selection","boundary_shapes_compared":["user_restated_boundary","obligation_grammar_boundary","verification_only_boundary"],"chosen_shape":"obligation_grammar_boundary","reason":"The goal requires delivered product outcomes plus proof and guardrails, so a proof-only boundary would shrink the run incorrectly."}}`,
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
