package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/gustian305/backend/config"
	"github.com/gustian305/backend/internal/evaluation"
	"github.com/gustian305/backend/internal/repository"
	"github.com/gustian305/backend/internal/service/consultation"
	expertSystem "github.com/gustian305/backend/internal/service/expert_system"
	expertLLM "github.com/gustian305/backend/internal/service/expert_system/llm"
	"github.com/gustian305/backend/internal/service/expert_system/pesticide"
	"github.com/gustian305/backend/internal/service/expert_system/rule"
)

func main() {
	var (
		configPath    = flag.String("config", "./config.yml", "path konfigurasi backend")
		outputRoot    = flag.String("output", "./evaluation-results", "direktori keluaran evaluasi")
		ruleCasesPath = flag.String("rule-cases", "./dataset/evaluation/rule_cases.json", "berkas 90 skenario tetap evaluasi rule engine")
		casesPath     = flag.String("llm-cases", "./dataset/evaluation/llm_cases.json", "berkas 40 skenario evaluasi LLM")
		expertRatings = flag.String("expert-ratings", "", "CSV penilaian pakar yang sudah diisi; kosong berarti hanya membuat template")
		liveLLM       = flag.Bool("live-llm", false, "panggil model LLM yang dikonfigurasi; tanpa flag ini evaluator memakai scripted guardrail")
		llmDelay      = flag.Duration("llm-delay", 300*time.Millisecond, "jeda antarpanggilan LLM live")
		strictExit    = flag.Bool("strict", true, "keluar dengan exit code 1 apabila target evaluasi tidak terpenuhi")
		timeout       = flag.Duration("timeout", 20*time.Minute, "batas waktu total evaluasi")
	)
	flag.Parse()

	// Mode evaluation tidak mewajibkan LLM key ketika hanya menjalankan suite
	// scripted. DATABASE_DSN tetap wajib karena rule dan rekomendasi diuji melalui
	// service produksi serta basis pengetahuan yang sama dengan aplikasi.
	if strings.TrimSpace(os.Getenv("APP_ENV")) == "" {
		_ = os.Setenv("APP_ENV", "evaluation")
	}

	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("gagal memuat konfigurasi: %v", err)
	}

	db, err := config.ConnectPostgres(cfg)
	if err != nil {
		log.Fatalf("gagal terhubung ke PostgreSQL: %v", err)
	}

	catalogRepo := repository.NewExpertCatalogRepository(db)
	safetyService := pesticide.NewSafetyService()
	pesticideResolver := pesticide.NewResolverService(catalogRepo, safetyService)
	ruleScorer := rule.NewScorerService()
	ruleService := rule.NewService(
		rule.NewMatcherService(catalogRepo, nil),
		ruleScorer,
		rule.NewResolverService(ruleScorer),
	)

	factory := func(client expertLLM.LLMClient) *expertSystem.RecommendationService {
		var llmService *expertLLM.RecommendationService
		if client != nil {
			llmService = expertLLM.NewRecommendationService(client)
		}
		return expertSystem.NewRecommendationService(
			catalogRepo,
			catalogRepo,
			catalogRepo,
			ruleService,
			pesticideResolver,
			safetyService,
			llmService,
		)
	}

	var liveClient expertLLM.LLMClient
	if *liveLLM {
		liveClient = consultation.NewOpenAICompatibleClient(
			cfg.LLM.APIURL,
			cfg.LLM.APIKey,
			cfg.LLM.Model,
			cfg.LLM.Timeout,
		)
		if liveClient == nil {
			log.Fatal("--live-llm dipilih, tetapi konfigurasi LLM belum lengkap; isi LLM_API_URL, LLM_API_KEY/GROQ_API_KEY, dan LLM_MODEL")
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	runner := evaluation.NewRunner(
		evaluation.Dependencies{
			DB:         db,
			Factory:    factory,
			LiveClient: liveClient,
			ModelName:  cfg.LLM.Model,
		},
		evaluation.Options{
			OutputRoot:        *outputRoot,
			RuleCasesPath:     *ruleCasesPath,
			LLMCasesPath:      *casesPath,
			ExpertRatingsPath: *expertRatings,
			LiveLLM:           *liveLLM,
			LLMDelay:          *llmDelay,
		},
	)

	result, err := runner.Run(ctx)
	if err != nil {
		log.Fatalf("evaluasi gagal dijalankan: %v", err)
	}

	fmt.Printf("\nEvaluasi selesai\n")
	fmt.Printf("Status              : %s\n", result.Summary.OverallStatus)
	fmt.Printf("Kelulusan keseluruhan: %.2f%% (%d lulus, %d gagal, %d skip)\n", result.Summary.PassRate, result.Summary.Passed, result.Summary.Failed, result.Summary.Skipped)
	fmt.Printf("Evaluasi LLM         : %d/%d (%.2f%%), target tercapai=%t\n", result.Summary.LLMCasePassed, result.Summary.LLMCaseTarget, result.Summary.LLMPassRate, result.Summary.LLMTargetMet)
	fmt.Printf("Kegagalan kritis     : %d\n", result.Summary.CriticalFailures)
	fmt.Printf("Rule macro F1        : %.4f\n", result.Summary.Metrics.RuleSelection.MacroF1Score)
	fmt.Printf("Rule accuracy        : %.4f\n", result.Summary.Metrics.RuleSelection.Accuracy)
	fmt.Printf("Decision macro F1    : %.4f\n", result.Summary.Metrics.DecisionGate.MacroF1Score)
	fmt.Printf("Validitas rekomendasi: %.2f%%\n", result.Summary.Metrics.Recommendation.CasePassRate*100)
	fmt.Printf("Keamanan akhir LLM   : %.2f%%\n", result.Summary.Metrics.LLM.FinalPolicySafetyRate*100)
	fmt.Printf("Halusinasi akhir LLM : %.2f%%\n", result.Summary.Metrics.LLM.FinalHallucinationRate*100)
	fmt.Printf("Keberhasilan fallback: %.2f%%\n", result.Summary.Metrics.LLM.FallbackSuccessRate*100)
	fmt.Printf("Dosis exact match    : %.2f%%\n", result.Summary.Metrics.Recommendation.DoseExactMatchRate*100)
	if result.Summary.Metrics.ExpertValidation.Available {
		fmt.Printf("Kelayakan pakar      : %.2f%%\n", result.Summary.Metrics.ExpertValidation.FeasibilityPercent)
	}
	fmt.Printf("Direktori hasil      : %s\n", result.OutputDir)
	for name, path := range result.Summary.Artifacts {
		fmt.Printf("- %-16s %s\n", name+":", path)
	}

	if *strictExit && result.Summary.OverallStatus != evaluation.StatusPass {
		os.Exit(1)
	}
}
