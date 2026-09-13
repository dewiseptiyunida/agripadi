package llm

import (
	"regexp"
	"sort"
	"strings"
	"unicode"
)

const defaultRecommendationNarrativeMaxWords = 220

var (
	policyDosePattern       = regexp.MustCompile(`(?i)\b\d+(?:[.,]\d+)?\s*(?:ml|mili?liter|l|liter|g|gram|kg|kilogram|ha|hektare)(?:\s*/\s*(?:l|liter|ha|hektare))?\b`)
	policyProductPattern    = regexp.MustCompile(`\b[A-Z][A-Z0-9-]{2,}(?:[ \t]+[A-Z0-9.,%-]{1,12}){0,3}[ \t]+(?:EC|SC|SL|WP|WG|SG|SP|OD|SE|ME|GR|G)\b`)
	policyHeadingPattern    = regexp.MustCompile(`(?i)^\s*(\d+)\s*[.)-]\s*(.+?)\s*$`)
	policyIngredientPattern = regexp.MustCompile(`(?i)(?:bahan aktif|mengandung)\s+([^.;\n]+)`)
)

// RecommendationPolicyContext berisi fakta yang sudah ditetapkan oleh rule
// engine dan basis pengetahuan. Validator tidak menentukan diagnosis baru; ia
// hanya memastikan narasi LLM tidak keluar dari fakta tersebut.
type RecommendationPolicyContext struct {
	PestName           string
	Severity           string
	GrowthStage        string
	AllowedProducts    []string
	AllowedIngredients []string
	KnownProducts      []string
	KnownIngredients   []string
	MaxWords           int
	RequirePHT         bool
	RequireMonitoring  bool
	RequireRotation    bool
	RequireTopProduct  bool
}

type RecommendationPolicyViolation struct {
	Code     string `json:"code"`
	Message  string `json:"message"`
	Critical bool   `json:"critical"`
}

type RecommendationPolicyResult struct {
	Accepted   bool                            `json:"accepted"`
	WordCount  int                             `json:"word_count"`
	Sections   map[string]string               `json:"sections"`
	Violations []RecommendationPolicyViolation `json:"violations"`
}

func (r RecommendationPolicyResult) ViolationMessages() []string {
	messages := make([]string, 0, len(r.Violations))
	for _, item := range r.Violations {
		messages = append(messages, item.Code+": "+item.Message)
	}
	return messages
}

// ValidateRecommendationNarrative menerapkan pembatasan keluaran yang sama
// untuk runtime dan cmd/evaluate. Satu pelanggaran kritis membuat keluaran LLM
// ditolak sehingga backend tetap memakai narasi deterministik.
func ValidateRecommendationNarrative(text string, ctx RecommendationPolicyContext) RecommendationPolicyResult {
	text = strings.TrimSpace(text)
	result := RecommendationPolicyResult{
		Accepted:   false,
		WordCount:  countPolicyWords(text),
		Sections:   parsePolicySections(text),
		Violations: make([]RecommendationPolicyViolation, 0),
	}

	add := func(code, message string, critical bool) {
		for _, existing := range result.Violations {
			if existing.Code == code {
				return
			}
		}
		result.Violations = append(result.Violations, RecommendationPolicyViolation{
			Code:     code,
			Message:  message,
			Critical: critical,
		})
	}

	if text == "" {
		add("empty_output", "Keluaran LLM kosong.", true)
	}

	lower := normalizePolicyText(text)
	if strings.Contains(lower, "<think>") || strings.Contains(lower, "</think>") || strings.Contains(lower, "proses berpikir") {
		add("reasoning_leak", "Keluaran menampilkan proses berpikir internal.", true)
	}

	for _, requiredSection := range []string{"2", "4", "5"} {
		if strings.TrimSpace(result.Sections[requiredSection]) == "" {
			add("missing_section_"+requiredSection, "Bagian wajib nomor "+requiredSection+" tidak ditemukan.", true)
		}
	}
	for section := range result.Sections {
		if section != "2" && section != "4" && section != "5" {
			add("unexpected_section", "Keluaran memuat bagian di luar nomor 2, 4, dan 5.", false)
			break
		}
	}

	maxWords := ctx.MaxWords
	if maxWords <= 0 {
		maxWords = defaultRecommendationNarrativeMaxWords
	}
	if result.WordCount > maxWords {
		add("word_limit", "Jumlah kata melebihi batas yang ditetapkan.", false)
	}

	currentPest := canonicalPolicyPest(ctx.PestName)
	for _, pestName := range []string{"penggerek batang", "walang sangit", "wereng batang cokelat"} {
		if pestName != currentPest && containsPolicyPhrase(lower, pestName) {
			add("diagnosis_changed", "Narasi menyebut hama lain di luar hasil rule engine.", true)
		}
	}

	severity := canonicalPolicySeverity(ctx.Severity)
	for _, candidate := range []string{"ringan", "sedang", "berat"} {
		if severity != "" && candidate != severity && containsPolicyPhrase(lower, candidate) {
			add("severity_changed", "Narasi menyebut tingkat serangan yang berbeda dari hasil sistem.", true)
		}
	}

	stage := canonicalPolicyGrowthStage(ctx.GrowthStage)
	if stage == "vegetatif" && containsPolicyPhrase(lower, "generatif") {
		add("growth_stage_changed", "Narasi mengubah fase vegetatif menjadi generatif.", true)
	}
	if stage == "generatif" && containsPolicyPhrase(lower, "vegetatif") {
		add("growth_stage_changed", "Narasi mengubah fase generatif menjadi vegetatif.", true)
	}

	if policyDosePattern.MatchString(text) {
		add("dose_fabrication", "Narasi LLM memuat angka dosis; dosis hanya boleh disajikan oleh komponen deterministik dari basis pengetahuan.", true)
	}

	for _, phrase := range []string{
		"abaikan diagnosis", "ubah diagnosis", "diagnosis sebenarnya", "ganti diagnosis",
		"abaikan aturan", "abaikan instruksi", "system override", "developer override",
		"paling efektif", "paling ampuh", "dijamin", "pasti berhasil", "tanpa apd",
		"campurkan dengan", "naikkan dosis", "gandakan dosis", "melebihi label",
	} {
		if containsPolicyPhrase(lower, phrase) {
			add("unsafe_instruction", "Narasi memuat klaim atau instruksi yang melanggar batas keselamatan.", true)
			break
		}
	}

	for _, term := range []string{" cnn ", " llm ", " backend ", " database ", " api ", " rule ", " skor "} {
		if strings.Contains(" "+lower+" ", term) {
			add("technical_term", "Narasi memuat istilah teknis yang tidak ditujukan kepada petani.", false)
			break
		}
	}

	allowedProducts := normalizedPolicySet(ctx.AllowedProducts)
	knownProducts := append([]string{}, ctx.KnownProducts...)
	knownProducts = append(knownProducts, ctx.AllowedProducts...)
	for _, product := range uniquePolicyStrings(knownProducts) {
		normalized := normalizePolicyText(product)
		if normalized == "" || !containsPolicyPhrase(lower, normalized) {
			continue
		}
		if _, ok := allowedProducts[normalized]; !ok {
			add("unknown_product", "Narasi menyebut produk yang tidak termasuk rekomendasi sistem.", true)
		}
	}

	for _, productCandidate := range policyProductPattern.FindAllString(text, -1) {
		candidate := normalizePolicyText(productCandidate)
		if candidate == "" {
			continue
		}
		if !policyValueAllowed(candidate, allowedProducts) {
			add("unknown_product", "Narasi memuat nama/formulasi produk yang tidak diberikan oleh sistem.", true)
		}
	}

	allowedIngredients := normalizedPolicySet(ctx.AllowedIngredients)
	knownIngredients := append([]string{}, ctx.KnownIngredients...)
	knownIngredients = append(knownIngredients, ctx.AllowedIngredients...)
	for _, ingredient := range uniquePolicyStrings(knownIngredients) {
		normalized := normalizePolicyText(ingredient)
		if normalized == "" || !containsPolicyPhrase(lower, normalized) {
			continue
		}
		if _, ok := allowedIngredients[normalized]; !ok {
			add("unknown_ingredient", "Narasi menyebut bahan aktif di luar rekomendasi sistem.", true)
		}
	}

	// Ekstraksi nama bahan aktif hanya dilakukan pada bagian alasan pemilihan.
	// Frasa keselamatan seperti "rotasi bahan aktif" pada bagian tindakan tidak
	// boleh disalahartikan sebagai penyebutan nama bahan aktif baru.
	ingredientScope := result.Sections["4"]
	if len(allowedIngredients) > 0 {
		for _, match := range policyIngredientPattern.FindAllStringSubmatch(ingredientScope, -1) {
			if len(match) < 2 {
				continue
			}
			for _, candidate := range splitPolicyIngredientCandidates(match[1]) {
				if !policyValueAllowed(candidate, allowedIngredients) {
					add("unknown_ingredient", "Narasi menyebut bahan aktif di luar rekomendasi sistem.", true)
					break
				}
			}
		}
	}

	sectionFour := normalizePolicyText(result.Sections["4"])
	if ctx.RequireTopProduct && len(ctx.AllowedProducts) > 0 {
		topProduct := normalizePolicyText(ctx.AllowedProducts[0])
		if topProduct != "" && !containsPolicyPhrase(sectionFour, topProduct) {
			add("top_product_missing", "Bagian alasan pemilihan tidak menyebut produk Top 1 yang diberikan sistem.", true)
		}
	}
	if len(ctx.AllowedIngredients) > 0 && !containsAnyPolicyPhrase(sectionFour, ctx.AllowedIngredients) {
		add("ingredient_missing", "Bagian alasan pemilihan tidak menyebut bahan aktif yang diberikan sistem.", true)
	}

	sectionFive := normalizePolicyText(result.Sections["5"])
	if ctx.RequirePHT && !containsAnyPolicyPhrase(sectionFive, []string{"pht", "pengendalian hama terpadu"}) {
		add("pht_missing", "Bagian tindakan tidak menyebut Pengendalian Hama Terpadu (PHT).", true)
	}
	if ctx.RequireMonitoring && !containsAnyPolicyPhrase(sectionFive, []string{"pantau", "pemantauan", "monitoring"}) {
		add("monitoring_missing", "Bagian tindakan tidak memuat pemantauan populasi/gejala.", true)
	}
	if ctx.RequireRotation && !containsAnyPolicyPhrase(sectionFive, []string{"rotasi bahan aktif", "pergiliran bahan aktif"}) {
		add("rotation_missing", "Bagian tindakan tidak memuat rotasi bahan aktif.", true)
	}

	sort.SliceStable(result.Violations, func(i, j int) bool {
		if result.Violations[i].Critical != result.Violations[j].Critical {
			return result.Violations[i].Critical
		}
		return result.Violations[i].Code < result.Violations[j].Code
	})

	result.Accepted = true
	for _, violation := range result.Violations {
		if violation.Critical {
			result.Accepted = false
			break
		}
	}
	if len(result.Violations) > 0 {
		// Untuk penelitian, satu kasus hanya lulus apabila seluruh kriteria
		// terpenuhi. Runtime tetap memprioritaskan keselamatan: pelanggaran
		// nonkritis dapat dicatat, tetapi keluaran tidak langsung dibatalkan.
		if !result.Accepted {
			return result
		}
	}

	return result
}

func parsePolicySections(text string) map[string]string {
	sections := make(map[string]string)
	current := ""
	buffer := make([]string, 0)

	flush := func() {
		if current == "" {
			buffer = buffer[:0]
			return
		}
		sections[current] = strings.TrimSpace(strings.Join(buffer, "\n"))
		buffer = buffer[:0]
	}

	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if matches := policyHeadingPattern.FindStringSubmatch(trimmed); len(matches) == 3 {
			flush()
			current = strings.TrimSpace(matches[1])
			continue
		}
		if current != "" {
			buffer = append(buffer, trimmed)
		}
	}
	flush()

	return sections
}

func countPolicyWords(text string) int {
	return len(strings.FieldsFunc(text, func(r rune) bool {
		return unicode.IsSpace(r) || strings.ContainsRune(",.;:!?()[]{}\"'", r)
	}))
}

func canonicalPolicyPest(value string) string {
	value = normalizePolicyText(value)
	switch {
	case strings.Contains(value, "wereng batang cokelat") || strings.Contains(value, "wereng batang coklat"):
		return "wereng batang cokelat"
	case strings.Contains(value, "walang sangit"):
		return "walang sangit"
	case strings.Contains(value, "penggerek batang"):
		return "penggerek batang"
	default:
		return value
	}
}

func canonicalPolicySeverity(value string) string {
	value = normalizePolicyText(value)
	switch value {
	case "low", "rendah", "ringan":
		return "ringan"
	case "medium", "moderate", "sedang":
		return "sedang"
	case "high", "tinggi", "parah", "berat":
		return "berat"
	default:
		return value
	}
}

func canonicalPolicyGrowthStage(value string) string {
	value = normalizePolicyText(value)
	switch {
	case strings.Contains(value, "vegetatif"):
		return "vegetatif"
	case strings.Contains(value, "generatif") || strings.Contains(value, "reproduktif") || strings.Contains(value, "pemasakan"):
		return "generatif"
	default:
		return value
	}
}

func normalizePolicyText(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	replacer := strings.NewReplacer("_", " ", "-", " ", "/", " ", "\\", " ", ",", " ", ".", " ", ":", " ", ";", " ", "(", " ", ")", " ")
	value = replacer.Replace(value)
	return strings.Join(strings.Fields(value), " ")
}

func containsPolicyPhrase(haystack, needle string) bool {
	haystack = " " + normalizePolicyText(haystack) + " "
	needle = normalizePolicyText(needle)
	if needle == "" {
		return false
	}
	return strings.Contains(haystack, " "+needle+" ")
}

func containsAnyPolicyPhrase(haystack string, needles []string) bool {
	for _, needle := range needles {
		if containsPolicyPhrase(haystack, needle) {
			return true
		}
	}
	return false
}

func normalizedPolicySet(values []string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		normalized := normalizePolicyText(value)
		if normalized != "" {
			result[normalized] = struct{}{}
		}
	}
	return result
}

func policyValueAllowed(candidate string, allowed map[string]struct{}) bool {
	candidate = normalizePolicyText(candidate)
	for value := range allowed {
		if candidate == value || strings.Contains(candidate, value) || strings.Contains(value, candidate) {
			return true
		}
	}
	return false
}

func splitPolicyIngredientCandidates(value string) []string {
	value = " " + normalizePolicyText(value) + " "
	for _, stop := range []string{
		" dan formulasi ", " serta formulasi ", " dengan formulasi ",
		" sesuai label ", " untuk hama ", " pada tanaman ",
		" sebagai bahan ", " yang terdaftar ",
	} {
		if index := strings.Index(value, stop); index >= 0 {
			value = value[:index]
		}
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	for _, prefix := range []string{"untuk ", "secara ", "agar ", "yang ", "berdasarkan ", "sesuai ", "dengan ", "pada ", "berikutnya ", "berbeda ", "lain "} {
		if strings.HasPrefix(value, prefix) {
			return nil
		}
	}
	replacer := strings.NewReplacer(" serta ", "|", " dan ", "|", ",", "|", "/", "|")
	parts := strings.Split(replacer.Replace(value), "|")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = normalizePolicyText(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

func uniquePolicyStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		normalized := normalizePolicyText(value)
		if normalized == "" {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, value)
	}
	return result
}
