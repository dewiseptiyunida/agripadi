package evaluation

import "time"

type Status string

const (
	StatusPass Status = "PASS"
	StatusFail Status = "FAIL"
	StatusSkip Status = "SKIP"
)

type Options struct {
	OutputRoot        string
	RuleCasesPath     string
	LLMCasesPath      string
	ExpertRatingsPath string
	LiveLLM           bool
	LLMDelay          time.Duration
}

type ExpertCaseDefinition struct {
	ID               string   `json:"id"`
	Category         string   `json:"category"`
	PestLabel        string   `json:"pest_label"`
	Severity         string   `json:"severity"`
	GrowthStage      string   `json:"growth_stage"`
	CNNConfidence    float64  `json:"cnn_confidence"`
	Symptoms         []string `json:"symptoms"`
	ExpectedRuleCode string   `json:"expected_rule_code,omitempty"`
	ExpectedFallback bool     `json:"expected_fallback"`
	Critical         bool     `json:"critical"`
}

type LLMCaseDefinition struct {
	ID                   string   `json:"id"`
	Category             string   `json:"category"`
	Description          string   `json:"description"`
	AttackText           string   `json:"attack_text,omitempty"`
	ForbiddenOutputTerms []string `json:"forbidden_output_terms,omitempty"`
	FailureMode          string   `json:"failure_mode,omitempty"`
}

type CaseResult struct {
	ID          string                 `json:"id"`
	Suite       string                 `json:"suite"`
	Category    string                 `json:"category"`
	Description string                 `json:"description"`
	Status      Status                 `json:"status"`
	Critical    bool                   `json:"critical"`
	Expected    string                 `json:"expected,omitempty"`
	Actual      string                 `json:"actual,omitempty"`
	DurationMS  float64                `json:"duration_ms"`
	Fallback    bool                   `json:"fallback,omitempty"`
	Violations  []string               `json:"violations,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

type SuiteSummary struct {
	Name      string  `json:"name"`
	Total     int     `json:"total"`
	Passed    int     `json:"passed"`
	Failed    int     `json:"failed"`
	Skipped   int     `json:"skipped"`
	PassRate  float64 `json:"pass_rate"`
	Criticals int     `json:"critical_failures"`
}

type Summary struct {
	RunID              string            `json:"run_id"`
	StartedAt          time.Time         `json:"started_at"`
	FinishedAt         time.Time         `json:"finished_at"`
	DurationSeconds    float64           `json:"duration_seconds"`
	Mode               string            `json:"mode"`
	Model              string            `json:"model"`
	DatabaseConnected  bool              `json:"database_connected"`
	OverallStatus      Status            `json:"overall_status"`
	Total              int               `json:"total"`
	Passed             int               `json:"passed"`
	Failed             int               `json:"failed"`
	Skipped            int               `json:"skipped"`
	PassRate           float64           `json:"pass_rate"`
	CriticalFailures   int               `json:"critical_failures"`
	LLMCaseTarget      int               `json:"llm_case_target"`
	LLMCasePassed      int               `json:"llm_case_passed"`
	LLMPassRate        float64           `json:"llm_pass_rate"`
	LLMTargetMet       bool              `json:"llm_target_met"`
	KnowledgeBaseStats map[string]int64  `json:"knowledge_base_stats,omitempty"`
	Suites             []SuiteSummary    `json:"suites"`
	Artifacts          map[string]string `json:"artifacts"`
	Metrics            MetricsReport     `json:"metrics"`
	Notes              []string          `json:"notes,omitempty"`
}

type RawLLMExchange struct {
	CaseID       string    `json:"case_id"`
	Category     string    `json:"category"`
	Model        string    `json:"model"`
	Prompt       string    `json:"prompt"`
	RawResponse  string    `json:"raw_response,omitempty"`
	Error        string    `json:"error,omitempty"`
	StartedAt    time.Time `json:"started_at"`
	DurationMS   float64   `json:"duration_ms"`
	FinalSource  string    `json:"final_source,omitempty"`
	FallbackUsed bool      `json:"fallback_used"`
}
