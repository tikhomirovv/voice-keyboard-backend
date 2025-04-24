package transcriber

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
)

// ConnecteAIService представляет собой сервис для транскрибации аудио с использованием Connecte.AI API
type ConnecteAIService struct {
	client  *http.Client
	config  *pkg.Config
	baseURL string
	token   string
}

// connecteAIRequest представляет собой запрос к Connecte.AI API
type connecteAIRequest struct {
	Audio         string `json:"audio"`          // URL аудиофайла
	Model         string `json:"model"`          // Модель для транскрибации
	Language      string `json:"language"`       // Язык речи
	Diarize       bool   `json:"diarize"`        // Разделение спикеров
	WordTimestamp bool   `json:"word_timestamp"` // Временные метки для слов
}

// connecteAIResponse представляет собой ответ от Connecte.AI API
type connecteAIResponse struct {
	ID     string            `json:"id"`
	Status string            `json:"status"`
	Output *connecteAIOutput `json:"output"`
	Cost   float64           `json:"cost"`
}

// connecteAIOutput содержит результат транскрибации
type connecteAIOutput struct {
	Text     string  `json:"text"`     // Текст транскрибации
	Duration float64 `json:"duration"` // Длительность аудио в секундах
	Language string  `json:"language"` // Код языка
}

// NewConnecteAIService создает новый экземпляр сервиса транскрибации с использованием Connecte.AI API
func NewConnecteAIService(config *pkg.Config) *ConnecteAIService {
	// Устанавливаем таймаут для HTTP-клиента (30 секунд)
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	return &ConnecteAIService{
		client:  client,
		config:  config,
		baseURL: config.Connecte.BaseUrl, // BaseUrl уже содержит /api/v1
		token:   config.Connecte.Token,
	}
}

// Transcribe выполняет транскрибацию аудио в текст
func (s *ConnecteAIService) Transcribe(ctx context.Context, request *dto.TranscriberRequest) (*dto.TranscriberResult, error) {
	// Получаем URL аудиофайла
	audioURL, err := s.buildAudioURL(request.UserID, request.SessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to build audio URL: %w", err)
	}

	// Создаем запрос к Connecte.AI API
	connecteRequest := &connecteAIRequest{
		Audio: audioURL,
		Model: "large-v3-turbo", // Используем самую быструю модель
		// Language:      "ru",             // Указываем русский язык для повышения точности
		Diarize:       false, // Отключаем разделение спикеров для ускорения
		WordTimestamp: false, // Отключаем временные метки для ускорения
	}

	fmt.Println("connecteRequest", connecteRequest)

	// Отправляем запрос к Connecte.AI API
	response, err := s.sendRequest(ctx, connecteRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to send request to Connecte.AI API: %w", err)
	}

	// Проверяем статус ответа
	if response.Status != "completed" {
		return nil, fmt.Errorf("transcription failed with status: %s", response.Status)
	}

	// Проверяем наличие результата
	if response.Output == nil {
		return nil, fmt.Errorf("transcription result is empty")
	}

	// Формируем результат транскрибации
	result := &dto.TranscriberResult{
		Text:         response.Output.Text,
		Duration:     response.Output.Duration,
		LanguageCode: response.Output.Language,
		// Здесь можно добавить расчет стоимости, если это необходимо
		Cost: response.Cost,
	}

	return result, nil
}

// buildAudioURL формирует URL аудиофайла
func (s *ConnecteAIService) buildAudioURL(userID uint64, sessionID string) (string, error) {
	// Формируем путь к аудиофайлу: base_url/api/v1/audio/:userID/:sessionID.wav
	audioPath := fmt.Sprintf("/api/v1/audio/%d/%s.wav", userID, sessionID)

	// Используем функцию из конфига для построения полного URL
	audioURL, err := s.config.BuildSiteUrl(audioPath, nil)
	if err != nil {
		return "", fmt.Errorf("failed to build audio URL: %w", err)
	}

	return audioURL, nil
}

// sendRequest отправляет запрос к Connecte.AI API и возвращает результат
func (s *ConnecteAIService) sendRequest(ctx context.Context, request *connecteAIRequest) (*connecteAIResponse, error) {
	// Формируем URL запроса
	url := fmt.Sprintf("%s/openai/whisper", s.baseURL)

	// Подготавливаем JSON-запрос
	reqBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Создаем HTTP-запрос
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Устанавливаем заголовки
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.token))

	// Выполняем запрос
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Проверяем код ответа
	if resp.StatusCode != http.StatusOK {
		// Читаем тело ответа для получения детальной информации об ошибке
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned non-OK status: %d, body: %s", resp.StatusCode, string(body))
	}

	// Декодируем ответ
	var response connecteAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &response, nil
}
