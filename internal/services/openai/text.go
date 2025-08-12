package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"gitlab.com/voice-keyboard/backend-go/internal/dto"
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
	apiRequestTimeout = 30 * time.Second
)

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
	request *dto.TextGenerationRequest,
) (*dto.TextGenerationResponse, error) {
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
	var response dto.TextGenerationResponse
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &response, nil
}

// CreateResponse отправляет запрос к Responses API
func (c *TextGenerationClient) CreateResponse(
	ctx context.Context,
	request *dto.ResponseRequest,
) (*dto.ResponseResult, error) {
	// Подготовка запроса
	reqBody, err := json.Marshal(request)
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
	var response dto.ResponseResult
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &response, nil
}

// OpenAITextGenerationService реализует интерфейс OpenAITextGenerationServiceInterface
type OpenAITextGenerationService struct {
	client *TextGenerationClient
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
		logger: logger,
	}
}

// GenerateText генерирует текст на основе промпта
func (s *OpenAITextGenerationService) GenerateText(
	ctx context.Context,
	request *dto.TextGenerationRequest,
) (*dto.TextGenerationResponse, error) {
	if request.Model == "" {
		request.Model = "gpt-4.1" // Модель по умолчанию
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
func (s *OpenAITextGenerationService) GenerateResponse(
	ctx context.Context,
	request *dto.ResponseRequest,
) (*dto.ResponseResult, error) {
	if request.Model == "" {
		request.Model = "gpt-4.1" // Модель по умолчанию
	}

	s.logger.Debug("Generating response with OpenAI Responses API",
		"model", request.Model,
		"input", request.Input)

	response, err := s.client.CreateResponse(ctx, request)
	if err != nil {
		s.logger.Error("Failed to generate response with OpenAI", "error", err)
		return nil, fmt.Errorf("response generation failed: %w", err)
	}

	// Извлекаем текст из ответа для логирования
	var outputText string
	if len(response.Output) > 0 && len(response.Output[0].Content) > 0 {
		outputText = response.Output[0].Content[0].Text
		if len(outputText) > 50 {
			outputText = outputText[:50] + "..." // Обрезаем для логов
		}
	}

	s.logger.Debug("Successfully generated response with OpenAI",
		"model", response.Model,
		"responseId", response.ID,
		"outputPreview", outputText)

	return response, nil
}

// ExtractTextFromResponse извлекает текст из ответа Responses API
// Возвращает пустую строку, если не удается найти текст в ответе
func (s *OpenAITextGenerationService) ExtractTextFromResponse(response *dto.ResponseResult) string {
	if response == nil {
		return ""
	}

	// Проверяем наличие сообщений в ответе
	if len(response.Output) == 0 {
		return ""
	}

	// Получаем первое сообщение
	message := response.Output[0]

	// Проверяем наличие содержимого в сообщении
	if len(message.Content) == 0 {
		return ""
	}

	// Ищем содержимое типа output_text
	for _, content := range message.Content {
		if content.Type == "output_text" {
			return content.Text
		}
	}

	// Возвращаем текст из первого содержимого, если не нашли output_text
	return message.Content[0].Text
}

// FixText исправляет текст на основе LLM
func (s *OpenAITextGenerationService) FixText(ctx context.Context, text string) (string, error) {
	if text == "" {
		return "", nil
	}

	prompt := fmt.Sprintf("Please correct all spelling, grammar, and punctuation in the following transcribed text. Add spaces, split sentences, and create new paragraphs where appropriate for better readability. Do not add, remove, or change any information. Do not paraphrase or interpret. Only return the corrected, well-formatted text. Text: `%s`", text)

	request := &dto.ResponseRequest{
		Model:       "gpt-4.1-nano",
		Input:       prompt,
		Temperature: 0.1,
	}

	response, err := s.GenerateResponse(ctx, request)
	if err != nil {
		return "", fmt.Errorf("[OpenAI] error generating response: %w", err)
	}

	return s.ExtractTextFromResponse(response), nil
}
