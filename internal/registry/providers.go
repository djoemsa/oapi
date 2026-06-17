package registry

// ProviderInfo represents the configuration defaults for an API provider.
type ProviderInfo struct {
	ID            string `json:"id" yaml:"id"`
	BaseURL       string `json:"base_url" yaml:"base_url"`
	DefaultRPM    int    `json:"default_rpm" yaml:"default_rpm"`
	DefaultRPD    int    `json:"default_rpd" yaml:"default_rpd"`
	ResetBehavior string `json:"reset_behavior" yaml:"reset_behavior"`
}

// ModelOverride represents per-model limits overrides (like Groq overrides).
type ModelOverride struct {
	ModelID string `json:"model_id" yaml:"model_id"`
	RPM     int    `json:"rpm" yaml:"rpm"`
	RPD     int    `json:"rpd" yaml:"rpd"`
	TPM     int    `json:"tpm" yaml:"tpm"`
	TPD     int    `json:"tpd" yaml:"tpd"`
}

// Default provider constants
const (
	ResetRolling24h = "rolling_24h"
	ResetMidnightPT = "midnight_pt"
	ResetContinuous = "continuous"
	ResetNone        = "none"
)

// Default providers registry map
var Providers = map[string]ProviderInfo{
	"groq": {
		ID:            "groq",
		BaseURL:       "https://api.groq.com/openai/v1",
		DefaultRPM:    30,
		DefaultRPD:    14400,
		ResetBehavior: ResetRolling24h,
	},
	"google": {
		ID:            "google",
		BaseURL:       "https://generativelanguage.googleapis.com/v1beta/openai",
		DefaultRPM:    15,
		DefaultRPD:    1500,
		ResetBehavior: ResetMidnightPT,
	},
	"cerebras": {
		ID:            "cerebras",
		BaseURL:       "https://api.cerebras.ai/v1",
		DefaultRPM:    5,
		DefaultRPD:    1000,
		ResetBehavior: ResetContinuous,
	},
	"github": {
		ID:            "github",
		BaseURL:       "https://models.inference.ai.azure.com",
		DefaultRPM:    15,
		DefaultRPD:    150,
		ResetBehavior: ResetRolling24h,
	},
	"openrouter": {
		ID:            "openrouter",
		BaseURL:       "https://openrouter.ai/api/v1",
		DefaultRPM:    20,
		DefaultRPD:    50,
		ResetBehavior: ResetMidnightPT,
	},
	"mistral": {
		ID:            "mistral",
		BaseURL:       "https://api.mistral.ai/v1",
		DefaultRPM:    300,
		DefaultRPD:    0, // No RPD limit, only TPM/RPM
		ResetBehavior: ResetNone,
	},
	"nvidia": {
		ID:            "nvidia",
		BaseURL:       "https://integrate.api.nvidia.com/v1",
		DefaultRPM:    10,
		DefaultRPD:    1000,
		ResetBehavior: ResetRolling24h,
	},
}

// Groq specific model overrides
var GroqModelOverrides = map[string]ModelOverride{
	"llama-3.1-8b-instant": {
		ModelID: "llama-3.1-8b-instant",
		RPM:     30,
		RPD:     14400,
		TPM:     6000,
		TPD:     500000,
	},
	"llama-3.3-70b-versatile": {
		ModelID: "llama-3.3-70b-versatile",
		RPM:     30,
		RPD:     1000,
		TPM:     12000,
		TPD:     100000,
	},
	"openai/gpt-oss-120b": {
		ModelID: "openai/gpt-oss-120b",
		RPM:     30,
		RPD:     1000,
		TPM:     8000,
		TPD:     200000,
	},
	"openai/gpt-oss-20b": {
		ModelID: "openai/gpt-oss-20b",
		RPM:     30,
		RPD:     1000,
		TPM:     8000,
		TPD:     200000,
	},
	"meta-llama/llama-4-scout-17b-16e-instruct": {
		ModelID: "meta-llama/llama-4-scout-17b-16e-instruct",
		RPM:     30,
		RPD:     1000,
		TPM:     30000,
		TPD:     500000,
	},
}

// Recommended models per provider
var RecommendedModels = map[string][]string{
	"groq":       {"llama-3.1-8b-instant", "llama-3.3-70b-versatile"},
	"google":     {"gemini-2.5-flash", "gemini-2.5-flash-001"},
	"cerebras":   {"gpt-oss-120b", "zai-glm-4.7"},
	"openrouter": {"openai/gpt-oss-120b:free", "meta-llama/llama-3.3-70b-instruct:free", "qwen/qwen3-next-80b-a3b-instruct:free"},
	"mistral":    {"mistral-small-2506", "ministral-3b-2512"},
	"github":     {"gpt-4o-mini", "Meta-Llama-3.1-70B-Instruct"},
	"nvidia":     {"meta/llama-4-maverick-17b-128e-instruct"},
}
