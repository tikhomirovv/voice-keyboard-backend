package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"gitlab.com/voice-keyboard/backend-go/pkg"
	"gitlab.com/voice-keyboard/backend-go/pkg/logger"
)

const (
	// Базовый URL для API OpenAI
	openAIBaseURL = "https://api.openai.com/v1"

	// URL для Chat Completions API
	chatCompletionsURL = openAIBaseURL + "/chat/completions"

	// URL для Responses API
	responsesURL = openAIBaseURL + "/responses"

	// Таймаут для запросов к API OpenAI
	apiRequestTimeout = 15 * time.Second
)

// isReasoningModel определяет, является ли модель reasoning-моделью
// Reasoning модели - это модели, которые содержат "gpt-5" в названии
func isReasoningModel(modelName string) bool {
	// Все модели, содержащие "gpt-5" в названии, считаются reasoning моделями
	return strings.Contains(strings.ToLower(modelName), "gpt-5")
}

// TextGenerationClient представляет клиент для работы с OpenAI API
type TextGenerationClient struct {
	apiKey     string
	httpClient *http.Client
	logger     logger.Logger
}

// NewTextGenerationClient создает новый клиент для работы с OpenAI API
func NewTextGenerationClient(apiKey string, logger logger.Logger) *TextGenerationClient {
	return &TextGenerationClient{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: apiRequestTimeout,
		},
		logger: logger,
	}
}

// CreateChatCompletion отправляет запрос к Chat Completions API
func (c *TextGenerationClient) CreateChatCompletion(
	ctx context.Context,
	request *TextGenerationRequest,
) (*TextGenerationResponse, error) {
	// Подготовка запроса
	reqBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Создание HTTP запроса
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, chatCompletionsURL, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Установка заголовков
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	// Отправка запроса
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Чтение ответа
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Проверка статуса ответа
	if resp.StatusCode != http.StatusOK {
		var errorResp struct {
			Error struct {
				Message string `json:"message"`
				Type    string `json:"type"`
			} `json:"error"`
		}
		if err := json.Unmarshal(respBody, &errorResp); err == nil && errorResp.Error.Message != "" {
			return nil, fmt.Errorf("openai api error: %s (type: %s)", errorResp.Error.Message, errorResp.Error.Type)
		}
		return nil, fmt.Errorf("failed request with status %d: %s", resp.StatusCode, string(respBody))
	}

	// Разбор ответа
	var response TextGenerationResponse
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &response, nil
}

// CreateResponse отправляет запрос к Responses API
// Автоматически определяет тип модели и использует соответствующие параметры
func (c *TextGenerationClient) CreateResponse(
	ctx context.Context,
	request *ResponseRequest,
) (*ResponseResult, error) {
	// Создаем правильную структуру в зависимости от типа модели
	var requestBody any

	if isReasoningModel(request.Model) {
		// Для reasoning-моделей
		requestBody = &ResponseRequestWithReasoning{
			ResponseRequest: *request,
			Reasoning: RequestReasoningFields{
				Effort: "minimal",
			},
		}
	} else {
		// Для обычных моделей
		requestBody = &ResponseRequestWithoutReasoning{
			ResponseRequest: *request,
			Temperature:     0.1,
		}
	}

	// Подготовка запроса
	reqBody, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Создание HTTP запроса
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, responsesURL, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Установка заголовков
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	// Отправка запроса
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Чтение ответа
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Проверка статуса ответа
	if resp.StatusCode != http.StatusOK {
		var errorResp struct {
			Error struct {
				Message string `json:"message"`
				Type    string `json:"type"`
			} `json:"error"`
		}
		if err := json.Unmarshal(respBody, &errorResp); err == nil && errorResp.Error.Message != "" {
			return nil, fmt.Errorf("openai api error: %s (type: %s)", errorResp.Error.Message, errorResp.Error.Type)
		}
		return nil, fmt.Errorf("failed request with status %d: %s", resp.StatusCode, string(respBody))
	}

	// Разбор ответа
	var response ResponseResult
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &response, nil
}

// OpenAITextGenerationService реализует интерфейс OpenAITextGenerationServiceInterface
type OpenAITextGenerationService struct {
	client *TextGenerationClient
	config *pkg.Config
	logger logger.Logger
}

// NewOpenAITextGenerationService создает новый сервис для генерации текста через OpenAI API
func NewOpenAITextGenerationService(
	config *pkg.Config,
	logger logger.Logger,
) *OpenAITextGenerationService {
	client := NewTextGenerationClient(config.OpenAI.APIKey, logger)

	return &OpenAITextGenerationService{
		client: client,
		config: config,
		logger: logger,
	}
}

// GenerateText генерирует текст на основе промпта
func (s *OpenAITextGenerationService) GenerateText(
	ctx context.Context,
	request *TextGenerationRequest,
) (*TextGenerationResponse, error) {
	if request.Model == "" {
		// Используем модель из конфигурации или модель по умолчанию
		if s.config.OpenAI.Model != "" {
			request.Model = s.config.OpenAI.Model
		} else {
			request.Model = "gpt-4.1-nano" // Модель по умолчанию
		}
	}

	s.logger.Debug("Generating text with OpenAI",
		"model", request.Model,
		"messagesCount", len(request.Messages))

	response, err := s.client.CreateChatCompletion(ctx, request)
	if err != nil {
		s.logger.Error("Failed to generate text with OpenAI", "error", err)
		return nil, fmt.Errorf("text generation failed: %w", err)
	}

	s.logger.Debug("Successfully generated text with OpenAI",
		"model", response.Model,
		"choicesCount", len(response.Choices))

	return response, nil
}

// GenerateResponse генерирует текст на основе инструкций и входных данных через Responses API
// Автоматически определяет тип модели (reasoning или обычная) и использует соответствующую структуру
func (s *OpenAITextGenerationService) GenerateResponse(
	ctx context.Context,
	request *ResponseRequest,
) (*ResponseResult, error) {
	if request.Model == "" {
		// Используем модель из конфигурации или модель по умолчанию
		if s.config.OpenAI.Model != "" {
			request.Model = s.config.OpenAI.Model
		} else {
			request.Model = "gpt-4.1-nano" // Модель по умолчанию
		}
	}

	s.logger.Debug("Generating response with OpenAI Responses API",
		"model", request.Model,
		"input", request.Input,
		"isReasoningModel", isReasoningModel(request.Model))

	response, err := s.client.CreateResponse(ctx, request)
	if err != nil {
		s.logger.Error("Failed to generate response with OpenAI", "error", err)
		return nil, fmt.Errorf("response generation failed: %w", err)
	}
	// Извлекаем текст из ответа для логирования
	extractedText := s.ExtractTextFromResponse(response)
	var outputPreview string
	if extractedText != "" {
		if len(extractedText) > 100 {
			outputPreview = extractedText[:100] + "..." // Обрезаем для логов
		} else {
			outputPreview = extractedText
		}
	}

	s.logger.Debug("Successfully generated response with OpenAI",
		"model", response.Model,
		"responseId", response.ID,
		"outputItemsCount", len(response.Output),
		"extractedText", outputPreview)

	return response, nil
}

// ExtractTextFromResponse извлекает текст из ответа Responses API
// Возвращает пустую строку, если не удается найти текст в ответе
func (s *OpenAITextGenerationService) ExtractTextFromResponse(response *ResponseResult) string {
	if response == nil {
		return ""
	}

	// Проверяем наличие сообщений в ответе
	if len(response.Output) == 0 {
		return ""
	}

	// Ищем сообщение с типом "message" (не "reasoning")
	for _, outputMessage := range response.Output {
		if outputMessage.Type == "message" && len(outputMessage.Content) > 0 {
			// Ищем содержимое типа output_text
			for _, content := range outputMessage.Content {
				if content.Type == "output_text" {
					return content.Text
				}
			}
			// Если не нашли output_text, возвращаем текст из первого содержимого
			if outputMessage.Content[0].Text != "" {
				return outputMessage.Content[0].Text
			}
		}
	}

	// Если не нашли message, попробуем взять из первого элемента (fallback для совместимости)
	message := response.Output[0]
	if len(message.Content) > 0 {
		for _, content := range message.Content {
			if content.Type == "output_text" {
				return content.Text
			}
		}
		if message.Content[0].Text != "" {
			return message.Content[0].Text
		}
	}

	return ""
}

// FixText исправляет текст на основе LLM
func (s *OpenAITextGenerationService) FixText(ctx context.Context, text string) (string, error) {
	if text == "" {
		return "", nil
	}

	prompt := fmt.Sprintf("Please correct all spelling, grammar, and punctuation in the following transcribed text. Add spaces, split sentences, and create new paragraphs where appropriate for better readability. Do not add, remove, or change any information. Do not paraphrase or interpret. Only return the corrected, well-formatted text. Text: `%s`", text)

	// Используем модель из конфигурации или модель по умолчанию
	model := s.config.OpenAI.Model
	if model == "" {
		model = "gpt-4.1-nano" // Модель по умолчанию
	}

	request := &ResponseRequest{
		Model: model,
		Input: prompt,
	}

	response, err := s.GenerateResponse(ctx, request)
	if err != nil {
		return "", fmt.Errorf("[OpenAI] error generating response: %w", err)
	}

	return s.ExtractTextFromResponse(response), nil
}
