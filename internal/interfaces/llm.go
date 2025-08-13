package interfaces

import "context"

// LLMTextGenerationServiceInterface определяет методы для работы с генерацией текста через OpenAI
type LLMTextGenerationServiceInterface interface {
	// FixText исправляет текст на основе LLM
	FixText(ctx context.Context, text string) (string, error)
}
