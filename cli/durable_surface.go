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
	DurableSurfaceObligationModel   DurableSurfaceName = "obligation-model"
	DurableSurfaceCognitionState    DurableSurfaceName = "cognition-state"
	DurableSurfaceAssurancePlan     DurableSurfaceName = "assurance-plan"
	DurableSurfaceCoordination      DurableSurfaceName = "coordination"
	DurableSurfaceStatus            DurableSurfaceName = "status"
	DurableSurfaceSuccessModel      DurableSurfaceName = "success-model"
	DurableSurfaceProofPlan         DurableSurfaceName = "proof-plan"
	DurableSurfaceWorkflowPlan      DurableSurfaceName = "workflow-plan"
	DurableSurfaceDomainPack        DurableSurfaceName = "domain-pack"
	DurableSurfaceCompilerInput     DurableSurfaceName = "compiler-input"
	DurableSurfaceCompilerReport    DurableSurfaceName = "compiler-report"
	DurableSurfaceImpactState       DurableSurfaceName = "impact-state"
	DurableSurfaceFreshnessState    DurableSurfaceName = "freshness-state"
	DurableSurfaceObligationLog     DurableSurfaceName = "obligation-log"
	DurableSurfaceEvidenceLog       DurableSurfaceName = "evidence-log"
	DurableSurfaceExperiments       DurableSurfaceName = "experiments"
	DurableSurfaceInterventionLog   DurableSurfaceName = "intervention-log"
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
			AuthoringFormat: DurableSurfaceSchemaFormatJSON,
			StorageFormat:   DurableSurfaceSchemaFormatJSON,
			Summary:         "Immutable extracted user-clause contract for this run.",
			Example:         `{"objective_hash":"sha256:demo","state":"locked","clauses":[{"id":"ucl-1","text":"Live trading works end to end on the live service.","kind":"delivery","source_excerpt":"所有功能端到端真实可用","required_surfaces":["obligation"],"approval_required_for_drop":true},{"id":"ucl-2","text":"Playwright user journey passes on the live service.","kind":"verification","source_excerpt":"Playwright 用户旅程测试全部通过（真实服务）","required_surfaces":["obligation","assurance"],"approval_required_for_drop":true}]}`,
			FieldNotes: []string{
				"`objective-contract` is immutable once `state` becomes `locked`.",
				"Each clause must keep a stable `id`, `text`, and `source_excerpt`.",
				"`kind` must stay within delivery|quality_bar|verification|guardrail|operating_rule.",
				"`required_surfaces` must stay within obligation|assurance.",
				"The framework enforces coverage integrity, not semantic satisfaction.",
			},
			FrameworkOwnedFields: []string{"`version`", "`created_at`", "`locked_at`"},
		},
		Path: ObjectiveContractPath,
	},
	DurableSurfaceObligationModel: {
		Name:               DurableSurfaceObligationModel,
		Class:              DurableSurfaceClassStructuredState,
		WriteMode:          DurableSurfaceWriteModeReplace,
		Strict:             true,
		FrameworkReadsBody: true,
		Schema: DurableSurfaceSchemaSpec{
			AuthoringFormat: DurableSurfaceSchemaFormatJSON,
			StorageFormat:   DurableSurfaceSchemaFormatJSON,
			Summary:         "Canonical mutable obligation boundary for the run, separate from execution decomposition and verification records.",
			Example:         `{"objective_contract_hash":"sha256:objective","required":[{"id":"obl-first-run","text":"A first-time operator can launch a run successfully.","kind":"outcome","covers_clauses":["ucl-first-run"],"assurance_required":true}],"optional":[],"guardrails":[{"id":"obl-no-corruption","text":"The run must not corrupt durable state.","kind":"guardrail","covers_clauses":["ucl-no-corruption"]}]}`,
			FieldNotes: []string{
				"`required` is the canonical mutable obligation boundary.",
				"`kind` must stay within outcome|enabler|proof|guardrail.",
				"Each obligation must include stable `id`, `text`, and `covers_clauses`.",
				"`assurance_required` records whether real assurance coverage is required.",
			},
			FrameworkOwnedFields: []string{"`version`", "`updated_at`"},
		},
		Path: ObligationModelPath,
	},
	DurableSurfaceCognitionState: {
		Name:               DurableSurfaceCognitionState,
		Class:              DurableSurfaceClassStructuredState,
		WriteMode:          DurableSurfaceWriteModeReplace,
		Strict:             true,
		FrameworkReadsBody: true,
		Schema: DurableSurfaceSchemaSpec{
			AuthoringFormat: DurableSurfaceSchemaFormatJSON,
			StorageFormat:   DurableSurfaceSchemaFormatJSON,
			Summary:         "Worktree-scoped cognition provider facts including invocation kind, index state, capabilities, and freshness anchors.",
			Example:         `{"scopes":[{"scope":"run-root","worktree_path":"/abs/path","providers":[{"name":"repo-native","invocation_kind":"builtin","available":true,"index_state":"fresh","head_revision":"def456","capabilities":["file_inventory","file_search","file_read","git_diff"]},{"name":"gitnexus","invocation_kind":"binary","command":"gitnexus","available":true,"version":"1.5.0","read_transports_supported":["cli","mcp"],"mcp_server_command":"gitnexus mcp","mcp_tools_supported":["list_repos","query","context","impact","detect_changes","rename"],"mcp_resources_supported":["gitnexus://repos","gitnexus://repo/{name}/context"],"registry_name":"demo-repo","index_state":"stale","index_provenance":"seeded","indexed_revision":"abc123","head_revision":"def456","stale_commits":2,"last_refresh_error":"status parse warning","analyzed_in_scope_at":"2026-04-01T00:00:00Z","capabilities":["query","context","impact","processes"]}]}]}`,
			FieldNotes: []string{
				"`scopes` is the canonical list of worktree cognition snapshots.",
				"`providers` records optional provider facts without semantic judgment.",
				"`invocation_kind` must stay explicit: builtin|binary|npx|mcp|none.",
				"`index_state` must stay explicit: missing|fresh|stale|unknown.",
				"`index_provenance` records whether graph cache is seeded or locally analyzed.",
				"`read_transports_supported` records provider-supported read transports such as cli and mcp without claiming active runtime ownership.",
				"`capabilities` is required for every provider entry.",
			},
			FrameworkOwnedFields: []string{"`version`", "`updated_at`"},
		},
		Path: CognitionStatePath,
	},
	DurableSurfaceAssurancePlan: {
		Name:               DurableSurfaceAssurancePlan,
		Class:              DurableSurfaceClassStructuredState,
		WriteMode:          DurableSurfaceWriteModeReplace,
		Strict:             true,
		FrameworkReadsBody: true,
		Schema: DurableSurfaceSchemaSpec{
			AuthoringFormat: DurableSurfaceSchemaFormatJSON,
			StorageFormat:   DurableSurfaceSchemaFormatJSON,
			Summary:         "Scenario-based external assurance strategy covering obligations, harnesses, oracles, evidence, and gate policy.",
			Example:         `{"obligation_refs":["obl-first-run"],"scenarios":[{"id":"scenario-cli-first-run","covers_obligations":["obl-first-run"],"harness":{"kind":"cli","command":"goalx run \"demo goal\""},"oracle":{"kind":"compound","checks":[{"kind":"exit_code","equals":"0"}]},"evidence":[{"kind":"stdout"},{"kind":"timing"}],"gate_policy":{"verify_lane":"required","required_cognition_tier":"repo-native","closeout":"required","merge":"required"}}]}`,
			FieldNotes: []string{
				"`scenarios` is the canonical scenario list.",
				"Each scenario must cover one or more obligations through `covers_obligations`.",
				"`required_cognition_tier` must stay within none|repo-native|graph.",
				"`verify_lane` must stay within quick|required|full when present.",
			},
			FrameworkOwnedFields: []string{"`version`", "`updated_at`"},
		},
		Path: AssurancePlanPath,
	},
	DurableSurfaceCoordination: {
		Name:               DurableSurfaceCoordination,
		Class:              DurableSurfaceClassStructuredState,
		WriteMode:          DurableSurfaceWriteModeReplace,
		Strict:             true,
		FrameworkReadsBody: true,
		Schema: DurableSurfaceSchemaSpec{
			AuthoringFormat: DurableSurfaceSchemaFormatJSON,
			StorageFormat:   DurableSurfaceSchemaFormatJSON,
			Summary:         "Short master-written coordination digest for required-item frontier state and session scope.",
			Example:         `{"plan_summary":["session-1 explores root cause"],"required":{"req-1":{"owner":"session-1","execution_state":"probing","surfaces":{"repo":"active","runtime":"pending","run_artifacts":"pending","web_research":"pending","external_system":"not_applicable"}}},"sessions":{"session-1":{"state":"active","scope":"trace unknown field source","last_round":1}},"decision":{"root_cause":"legacy schema drift","chosen_path":"single_source_contract","chosen_path_reason":"one concern one path"},"open_questions":[]}`,
			FieldNotes: []string{
				"No legacy aliases are accepted for session grouping fields.",
				"`required` is the canonical required-item frontier map.",
				"Coverage derives `premature_blocked` when a `blocked` item still has non-terminal machine surfaces.",
				"Keep verbose reasoning in journals, not this digest.",
			},
			FrameworkOwnedFields: []string{"`version`", "`updated_at`"},
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
			AuthoringFormat: DurableSurfaceSchemaFormatJSON,
			StorageFormat:   DurableSurfaceSchemaFormatJSON,
			Summary:         "Master-written run progress facts for phase, remaining required work, and ownership.",
			Example:         `{"phase":"working","required_remaining":0,"open_required_ids":["req-1"],"active_sessions":["session-1"],"keep_session":"session-2","last_verified_at":"2026-03-28T10:00:00Z"}`,
			FieldNotes: []string{
				"`required_remaining` is required and must be non-negative.",
				"`phase` is restricted to working|review|complete.",
				"`open_required_ids` and `active_sessions` are optional factual snapshots when the master explicitly records them.",
				"No recommendation or completion verdict fields are accepted.",
			},
			FrameworkOwnedFields: []string{"`version`", "`updated_at`"},
		},
		Path: RunStatusPath,
	},
	DurableSurfaceSuccessModel: {
		Name:               DurableSurfaceSuccessModel,
		Class:              DurableSurfaceClassStructuredState,
		WriteMode:          DurableSurfaceWriteModeReplace,
		Strict:             true,
		FrameworkReadsBody: true,
		Schema: DurableSurfaceSchemaSpec{
			AuthoringFormat: DurableSurfaceSchemaFormatJSON,
			StorageFormat:   DurableSurfaceSchemaFormatJSON,
			Summary:         "Compiled success-model surface defining required quality dimensions, anti-goals, and structural closeout requirements for this run.",
			Example:         `{"objective_contract_hash":"sha256:objective","obligation_model_hash":"sha256:obligation","dimensions":[{"id":"dim-product-clarity","kind":"quality","text":"Operators can orient within seconds.","required":true,"failure_modes":["correct_but_unclear"]}],"anti_goals":[{"id":"anti-proof-only","text":"Do not treat proof-only success as sufficient."}],"closeout_requirements":["quality_debt_zero"]}`,
			FieldNotes: []string{
				"`dimensions` is the canonical success-dimension list for the run.",
				"Each dimension must include stable `id`, `kind`, and `text` fields.",
				"`anti_goals` records explicit anti-optimizations the runtime should keep visible.",
				"`closeout_requirements` is structural, not semantic scoring.",
			},
			FrameworkOwnedFields: []string{"`version`", "`compiled_at`"},
		},
		Path: SuccessModelPath,
	},
	DurableSurfaceProofPlan: {
		Name:               DurableSurfaceProofPlan,
		Class:              DurableSurfaceClassStructuredState,
		WriteMode:          DurableSurfaceWriteModeReplace,
		Strict:             true,
		FrameworkReadsBody: true,
		Schema: DurableSurfaceSchemaSpec{
			AuthoringFormat: DurableSurfaceSchemaFormatJSON,
			StorageFormat:   DurableSurfaceSchemaFormatJSON,
			Summary:         "Compiled proof-plan surface defining what proof forms structurally cover each success dimension.",
			Example:         `{"items":[{"id":"proof-correctness","covers_dimensions":["dim-correctness"],"kind":"assurance_check","required":true,"source_surface":"assurance"},{"id":"proof-product-clarity-visual","covers_dimensions":["dim-product-clarity"],"kind":"visual_evidence","required":true,"source_surface":"artifact"}]}`,
			FieldNotes: []string{
				"Each item must cover one or more success dimensions through `covers_dimensions`.",
				"`kind` describes the required proof form, not the semantic verdict.",
				"`source_surface` identifies where the proof is expected to land.",
			},
			FrameworkOwnedFields: []string{"`version`", "`compiled_at`"},
		},
		Path: ProofPlanPath,
	},
	DurableSurfaceWorkflowPlan: {
		Name:               DurableSurfaceWorkflowPlan,
		Class:              DurableSurfaceClassStructuredState,
		WriteMode:          DurableSurfaceWriteModeReplace,
		Strict:             true,
		FrameworkReadsBody: true,
		Schema: DurableSurfaceSchemaSpec{
			AuthoringFormat: DurableSurfaceSchemaFormatJSON,
			StorageFormat:   DurableSurfaceSchemaFormatJSON,
			Summary:         "Compiled workflow-plan surface defining which runtime roles and structural gates must exist before success can be claimed.",
			Example:         `{"required_roles":[{"id":"builder","required":true},{"id":"critic","required":true},{"id":"finisher","required":true}],"gates":["builder_result_present","critic_review_present","finisher_pass_present"]}`,
			FieldNotes: []string{
				"`required_roles` defines the minimal runtime role set for this run.",
				"`gates` lists structural workflow checkpoints, not semantic scores.",
			},
			FrameworkOwnedFields: []string{"`version`", "`compiled_at`"},
		},
		Path: WorkflowPlanPath,
	},
	DurableSurfaceDomainPack: {
		Name:               DurableSurfaceDomainPack,
		Class:              DurableSurfaceClassStructuredState,
		WriteMode:          DurableSurfaceWriteModeReplace,
		Strict:             true,
		FrameworkReadsBody: true,
		Schema: DurableSurfaceSchemaSpec{
			AuthoringFormat: DurableSurfaceSchemaFormatJSON,
			StorageFormat:   DurableSurfaceSchemaFormatJSON,
			Summary:         "Run-scoped composed snapshot of the repo policy, learned priors, and explicit run-context slots compiled into this run.",
			Example:         `{"domain":"frontend_product","signals":["operator_console","quality_ambiguous"],"slots":{"repo_policy":{"source":"AGENTS.md","refs":["AGENTS.md"]},"learned_success_priors":{"entry_ids":["mem_success_1"]},"run_context":{"source":"control/memory-context.json","refs":["control/memory-context.json"]}},"prior_entry_ids":["mem_success_1"]}`,
			FieldNotes: []string{
				"`domain-pack` is a compiled run artifact, not canonical memory.",
				"`slots` records which source class filled which composed pack slot.",
				"`prior_entry_ids` references the exact memory entries used for this run snapshot.",
			},
			FrameworkOwnedFields: []string{"`version`", "`compiled_at`"},
		},
		Path: DomainPackPath,
	},
	DurableSurfaceCompilerInput: {
		Name:               DurableSurfaceCompilerInput,
		Class:              DurableSurfaceClassStructuredState,
		WriteMode:          DurableSurfaceWriteModeReplace,
		Strict:             true,
		FrameworkReadsBody: true,
		Schema: DurableSurfaceSchemaSpec{
			AuthoringFormat: DurableSurfaceSchemaFormatJSON,
			StorageFormat:   DurableSurfaceSchemaFormatJSON,
			Summary:         "Frozen run-scoped snapshot of the compiler inputs used to build the success plane.",
			Example:         `{"objective_contract_ref":"objective-contract.json","obligation_model_ref":"obligation-model.json","memory_query_ref":"control/memory-query.json","memory_context_ref":"control/memory-context.json","policy_source_refs":["AGENTS.md"],"selected_prior_refs":["mem_success_1"],"source_slots":[{"slot":"repo_policy","refs":["AGENTS.md"]},{"slot":"learned_success_priors","refs":["mem_success_1"]},{"slot":"run_context","refs":["control/memory-context.json"]}]}`,
			FieldNotes: []string{
				"`compiler-input` freezes the exact durable input graph used for this run compilation.",
				"`source_slots` is restricted to repo_policy|learned_success_priors|run_context.",
				"`selected_prior_refs` lists the memory entries the compiler actually selected.",
			},
			FrameworkOwnedFields: []string{"`version`", "`compiled_at`", "`compiler_version`"},
		},
		Path: CompilerInputPath,
	},
	DurableSurfaceCompilerReport: {
		Name:               DurableSurfaceCompilerReport,
		Class:              DurableSurfaceClassStructuredState,
		WriteMode:          DurableSurfaceWriteModeReplace,
		Strict:             true,
		FrameworkReadsBody: true,
		Schema: DurableSurfaceSchemaSpec{
			AuthoringFormat: DurableSurfaceSchemaFormatJSON,
			StorageFormat:   DurableSurfaceSchemaFormatJSON,
			Summary:         "Structured compiler explanation showing selected sources, rejected priors, and output-to-source mappings.",
			Example:         `{"available_source_slots":[{"slot":"repo_policy","refs":["AGENTS.md"]},{"slot":"learned_success_priors","refs":["mem_success_1"]}],"selected_prior_refs":["mem_success_1"],"rejected_priors":[{"ref":"mem_success_2","reason_code":"no_selector_match"}],"output_sources":[{"output":"success-model.dimension:dim-objective","source_slot":"repo_policy","refs":["AGENTS.md"]}]}`,
			FieldNotes: []string{
				"`compiler-report` explains compiler choices without adding semantic verdicts.",
				"`rejected_priors.reason_code` is restricted to explicit reason codes.",
				"`output_sources` maps compiled outputs back to source slots and refs.",
			},
			FrameworkOwnedFields: []string{"`version`", "`compiled_at`", "`compiler_version`"},
		},
		Path: CompilerReportPath,
	},
	DurableSurfaceImpactState: {
		Name:               DurableSurfaceImpactState,
		Class:              DurableSurfaceClassStructuredState,
		WriteMode:          DurableSurfaceWriteModeReplace,
		Strict:             true,
		FrameworkReadsBody: true,
		Schema: DurableSurfaceSchemaSpec{
			AuthoringFormat: DurableSurfaceSchemaFormatJSON,
			StorageFormat:   DurableSurfaceSchemaFormatJSON,
			Summary:         "Latest code-change observation for one scope, including baseline/head revisions and resolved touchpoints.",
			Example:         `{"scope":"run-root","baseline_revision":"abc123","head_revision":"def456","resolver_kind":"repo-native","changed_files":["cli/start.go"],"changed_symbols":["cli.Start"],"changed_processes":["run_bootstrap"]}`,
			FieldNotes: []string{
				"`resolver_kind` must stay explicit: repo-native|gitnexus|file_only|none.",
				"`baseline_revision` and `head_revision` are required.",
				"`changed_files`, `changed_symbols`, and `changed_processes` are factual observations, not verdicts.",
			},
			FrameworkOwnedFields: []string{"`version`", "`updated_at`"},
		},
		Path: ImpactStatePath,
	},
	DurableSurfaceFreshnessState: {
		Name:               DurableSurfaceFreshnessState,
		Class:              DurableSurfaceClassStructuredState,
		WriteMode:          DurableSurfaceWriteModeReplace,
		Strict:             true,
		FrameworkReadsBody: true,
		Schema: DurableSurfaceSchemaSpec{
			AuthoringFormat: DurableSurfaceSchemaFormatJSON,
			StorageFormat:   DurableSurfaceSchemaFormatJSON,
			Summary:         "Freshness facts for cognition providers and recorded scenario evidence.",
			Example:         `{"cognition":[{"scope":"run-root","provider":"repo-native","state":"fresh"}],"evidence":[{"scenario_id":"scenario-cli-first-run","latest_revision":"abc123","current_revision":"def456","state":"stale","reason":"changed_process_overlap=run_bootstrap"}]}`,
			FieldNotes: []string{
				"`state` must stay within fresh|stale|unknown|not_applicable.",
				"`cognition` records provider freshness facts.",
				"`evidence` records scenario evidence freshness facts.",
			},
			FrameworkOwnedFields: []string{"`version`", "`updated_at`"},
		},
		Path: FreshnessStatePath,
	},
	DurableSurfaceObligationLog: {
		Name:               DurableSurfaceObligationLog,
		Class:              DurableSurfaceClassEventLog,
		WriteMode:          DurableSurfaceWriteModeAppend,
		Strict:             true,
		FrameworkReadsBody: false,
		Schema: DurableSurfaceSchemaSpec{
			AuthoringFormat: DurableSurfaceSchemaFormatJSON,
			StorageFormat:   DurableSurfaceSchemaFormatJSONL,
			Summary:         "Append-only obligation boundary events. The framework serializes the canonical JSONL envelope.",
			Example:         `{"obligation_model_version":2,"decision":"initial_obligation_boundary","chosen_shape":"obligation-model","reason":"Required outcomes, guardrails, and assurance lanes need separate canonical ownership."}`,
			FieldNotes: []string{
				"`--kind` and `--actor` are required on the write command.",
				"`--body-file` must contain one JSON object representing the event body.",
				"The framework writes the canonical JSONL envelope and timestamp.",
			},
			FrameworkOwnedFields: []string{"storage envelope `version`", "storage envelope `at`", "storage envelope `kind`", "storage envelope `actor`"},
			AllowedKinds:         []string{"decision", "checkpoint", "waiver", "closeout", "update"},
		},
		Path: ObligationLogPath,
	},
	DurableSurfaceEvidenceLog: {
		Name:               DurableSurfaceEvidenceLog,
		Class:              DurableSurfaceClassEventLog,
		WriteMode:          DurableSurfaceWriteModeAppend,
		Strict:             true,
		FrameworkReadsBody: false,
		Schema: DurableSurfaceSchemaSpec{
			AuthoringFormat: DurableSurfaceSchemaFormatJSON,
			StorageFormat:   DurableSurfaceSchemaFormatJSONL,
			Summary:         "Append-only scenario evidence events for assurance runs and judge/signoff recordings.",
			Example:         `{"scenario_id":"scenario-cli-first-run","scope":"run-root","revision":"def456","harness_kind":"cli","oracle_result":{"exit_code":0},"artifact_refs":["reports/assurance/scenario-cli-first-run/stdout.txt"]}`,
			FieldNotes: []string{
				"`--kind` and `--actor` are required on the write command.",
				"`scenario_id` and `harness_kind` are required in the body.",
				"The framework stores the canonical JSONL envelope; closeout interpretation remains agent-owned.",
			},
			FrameworkOwnedFields: []string{"storage envelope `version`", "storage envelope `at`", "storage envelope `kind`", "storage envelope `actor`"},
			AllowedKinds:         []string{"scenario.executed", "judge.recorded", "signoff.recorded"},
		},
		Path: EvidenceLogPath,
	},
	DurableSurfaceExperiments: {
		Name:               DurableSurfaceExperiments,
		Class:              DurableSurfaceClassEventLog,
		WriteMode:          DurableSurfaceWriteModeAppend,
		Strict:             true,
		FrameworkReadsBody: false,
		Schema: DurableSurfaceSchemaSpec{
			AuthoringFormat: DurableSurfaceSchemaFormatJSON,
			StorageFormat:   DurableSurfaceSchemaFormatJSONL,
			Summary:         "Append-only experiment lineage events. The framework serializes the canonical JSONL envelope and per-kind event timestamp fields.",
			Example:         `{"experiment_id":"exp-1","session":"session-1","branch":"goalx/demo/root","worktree":"/abs/run/worktrees/demo-root","intent":"evolve","base_ref":"goalx/demo/root"}`,
			FieldNotes: []string{
				"`--kind` and `--actor` are required on the write command.",
				"`--body-file` must contain one JSON object matching the chosen experiment event kind.",
				"Do not infer verdicts from `body`; interpretation belongs to agents.",
			},
			FrameworkOwnedFields: []string{"storage envelope `version`", "storage envelope `at`", "derived body timestamps like `created_at` / `recorded_at` / `closed_at` / `stopped_at`"},
			AllowedKinds:         []string{"experiment.created", "experiment.integrated", "experiment.closed", "evolve.stopped"},
		},
		Path: ExperimentsLogPath,
	},
	DurableSurfaceInterventionLog: {
		Name:               DurableSurfaceInterventionLog,
		Class:              DurableSurfaceClassEventLog,
		WriteMode:          DurableSurfaceWriteModeAppend,
		Strict:             true,
		FrameworkReadsBody: false,
		Schema: DurableSurfaceSchemaSpec{
			AuthoringFormat: DurableSurfaceSchemaFormatJSON,
			StorageFormat:   DurableSurfaceSchemaFormatJSONL,
			Summary:         "Append-only intervention event log for high-value user or master redirects that may later generate success-delta proposals.",
			Example:         `{"message":"Do not stop at route cutover only.","before":{"obligation_model_hash":"sha256:obligation","status_hash":"sha256:status","coordination_hash":"sha256:coordination","success_model_hash":"sha256:success"},"affected_targets":["session-3","req-p4-web-cockpit"]}`,
			FieldNotes: []string{
				"`--kind` and `--actor` are required on the write command.",
				"`message` captures the intervention text; richer evidence remains in linked reports or memory proposals.",
				"The framework stores the canonical JSONL envelope; extraction and promotion happen elsewhere.",
			},
			FrameworkOwnedFields: []string{"storage envelope `version`", "storage envelope `at`", "storage envelope `kind`", "storage envelope `actor`"},
			AllowedKinds:         []string{"user_redirect", "user_tell", "master_reframe", "budget_extend", "budget_set_total", "budget_clear"},
		},
		Path: InterventionLogPath,
	},
	DurableSurfaceSummary: {
		Name:               DurableSurfaceSummary,
		Class:              DurableSurfaceClassArtifact,
		WriteMode:          DurableSurfaceWriteModeReplace,
		Strict:             false,
		FrameworkReadsBody: false,
		Schema: DurableSurfaceSchemaSpec{
			AuthoringFormat: DurableSurfaceSchemaFormatMarkdown,
			StorageFormat:   DurableSurfaceSchemaFormatMarkdown,
			Summary:         "Opaque closeout summary artifact for user-facing result narrative.",
			Example:         "# Summary\n\n- Outcome\n- Evidence links\n- Next steps\n",
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
			AuthoringFormat: DurableSurfaceSchemaFormatJSON,
			StorageFormat:   DurableSurfaceSchemaFormatJSON,
			Summary:         "Opaque completion-proof artifact authored by the master.",
			Example:         `{"notes":"master-owned completion interpretation artifact"}`,
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
