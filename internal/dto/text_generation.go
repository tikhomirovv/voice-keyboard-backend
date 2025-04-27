package dto

// Message представляет сообщение для генерации текста в формате OpenAI
type Message struct {
	Role    string `json:"role" validate:"required,oneof=user assistant developer"`
	Content string `json:"content" validate:"required"`
}

// TextGenerationRequest представляет запрос на генерацию текста
type TextGenerationRequest struct {
	// Model - название модели OpenAI для генерации текста
	Model string `json:"model" validate:"required"`

	// Messages - массив сообщений для генерации текста
	Messages []Message `json:"messages" validate:"required,dive"`

	// MaxTokens - максимальное количество токенов в ответе
	MaxTokens int `json:"max_tokens,omitempty"`

	// Temperature - параметр случайности (0.0-2.0)
	Temperature float32 `json:"temperature,omitempty" validate:"omitempty,min=0,max=2"`

	// TopP - параметр вероятности следующего токена (0.0-1.0)
	TopP float32 `json:"top_p,omitempty" validate:"omitempty,min=0,max=1"`
}

// TextGenerationChoice представляет один вариант сгенерированного текста
type TextGenerationChoice struct {
	Index   int     `json:"index"`
	Message Message `json:"message"`
}

// TextGenerationResponse представляет ответ от сервиса генерации текста
type TextGenerationResponse struct {
	ID      string                 `json:"id"`
	Object  string                 `json:"object"`
	Created int64                  `json:"created"`
	Model   string                 `json:"model"`
	Choices []TextGenerationChoice `json:"choices"`
	Usage   TextGenerationUsage    `json:"usage"`
}

// TextGenerationUsage представляет информацию об использовании токенов
type TextGenerationUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ResponseRequest представляет запрос к Responses API
type ResponseRequest struct {
	// Model - название модели OpenAI для генерации текста
	Model string `json:"model" validate:"required"`

	// Instructions - инструкции для модели (опционально)
	Instructions string `json:"instructions,omitempty"`

	// Input - входные данные для запроса
	Input string `json:"input" validate:"required"`

	// Temperature - параметр случайности (0.0-2.0)
	Temperature float32 `json:"temperature,omitempty" validate:"omitempty,min=0,max=2"`

	// MaxOutputTokens - максимальное количество токенов в ответе
	MaxOutputTokens int `json:"max_output_tokens,omitempty"`

	// TopP - параметр вероятности следующего токена (0.0-1.0)
	TopP float32 `json:"top_p,omitempty" validate:"omitempty,min=0,max=1"`
}

// OutputContent представляет содержимое ответа
type OutputContent struct {
	Type        string        `json:"type"`
	Text        string        `json:"text"`
	Annotations []interface{} `json:"annotations"`
}

// OutputMessage представляет сообщение в ответе от Responses API
type OutputMessage struct {
	Type    string          `json:"type"`
	ID      string          `json:"id"`
	Status  string          `json:"status"`
	Role    string          `json:"role"`
	Content []OutputContent `json:"content"`
}

// ResponseUsage представляет информацию об использовании токенов в Responses API
type ResponseUsage struct {
	InputTokens         int            `json:"input_tokens"`
	OutputTokens        int            `json:"output_tokens"`
	TotalTokens         int            `json:"total_tokens"`
	InputTokensDetails  map[string]any `json:"input_tokens_details"`
	OutputTokensDetails map[string]any `json:"output_tokens_details"`
}

// ResponseResult представляет ответ от Responses API
type ResponseResult struct {
	ID                 string          `json:"id"`
	Object             string          `json:"object"`
	CreatedAt          int64           `json:"created_at"`
	Status             string          `json:"status"`
	Error              any             `json:"error"`
	IncompleteDetails  any             `json:"incomplete_details"`
	Instructions       any             `json:"instructions"`
	MaxOutputTokens    any             `json:"max_output_tokens"`
	Model              string          `json:"model"`
	Output             []OutputMessage `json:"output"`
	ParallelToolCalls  bool            `json:"parallel_tool_calls"`
	PreviousResponseID any             `json:"previous_response_id"`
	Store              bool            `json:"store"`
	Temperature        float32         `json:"temperature"`
	Text               map[string]any  `json:"text"`
	ToolChoice         string          `json:"tool_choice"`
	Tools              []any           `json:"tools"`
	TopP               float32         `json:"top_p"`
	Truncation         string          `json:"truncation"`
	Usage              ResponseUsage   `json:"usage"`
	User               any             `json:"user"`
	Metadata           map[string]any  `json:"metadata"`
}
