package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/shared"
	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/llm/models"
	"github.com/opencode-ai/opencode/internal/llm/tools"
	"github.com/opencode-ai/opencode/internal/logging"
	"github.com/opencode-ai/opencode/internal/message"
)

type openaiOptions struct {
	baseURL         string
	disableCache    bool
	reasoningEffort string
	extraHeaders    map[string]string
	temperature     *float64
	topP            *float64
	minP            *float64
	topK            *int64
	numCtx          *int64
	repeatPenalty   *float64
	seed            *int64
}

type OpenAIOption func(*openaiOptions)

type openaiClient struct {
	providerOptions providerClientOptions
	options         openaiOptions
	client          openai.Client
}

type OpenAIClient ProviderClient

var localModelNoToolsCache sync.Map

func newOpenAIClient(opts providerClientOptions) OpenAIClient {
	openaiOpts := openaiOptions{
		reasoningEffort: "medium",
	}
	for _, o := range opts.openaiOptions {
		o(&openaiOpts)
	}

	openaiClientOptions := []option.RequestOption{}
	if opts.apiKey != "" {
		openaiClientOptions = append(openaiClientOptions, option.WithAPIKey(opts.apiKey))
	}
	if openaiOpts.baseURL != "" {
		openaiClientOptions = append(openaiClientOptions, option.WithBaseURL(openaiOpts.baseURL))
	}

	if openaiOpts.extraHeaders != nil {
		for key, value := range openaiOpts.extraHeaders {
			openaiClientOptions = append(openaiClientOptions, option.WithHeader(key, value))
		}
	}

	client := openai.NewClient(openaiClientOptions...)
	return &openaiClient{
		providerOptions: opts,
		options:         openaiOpts,
		client:          client,
	}
}

func (o *openaiClient) convertMessages(messages []message.Message) (openaiMessages []openai.ChatCompletionMessageParamUnion) {
	// Add system message first
	openaiMessages = append(openaiMessages, openai.SystemMessage(o.providerOptions.systemMessage))

	for _, msg := range messages {
		switch msg.Role {
		case message.User:
			var content []openai.ChatCompletionContentPartUnionParam
			textBlock := openai.ChatCompletionContentPartTextParam{Text: msg.Content().String()}
			content = append(content, openai.ChatCompletionContentPartUnionParam{OfText: &textBlock})
			for _, binaryContent := range msg.BinaryContent() {
				imageURL := openai.ChatCompletionContentPartImageImageURLParam{URL: binaryContent.String(models.ProviderOpenAI)}
				imageBlock := openai.ChatCompletionContentPartImageParam{ImageURL: imageURL}

				content = append(content, openai.ChatCompletionContentPartUnionParam{OfImageURL: &imageBlock})
			}

			openaiMessages = append(openaiMessages, openai.UserMessage(content))

		case message.Assistant:
			assistantMsg := openai.ChatCompletionAssistantMessageParam{
				Role: "assistant",
			}

			if msg.Content().String() != "" {
				assistantMsg.Content = openai.ChatCompletionAssistantMessageParamContentUnion{
					OfString: openai.String(msg.Content().String()),
				}
			}

			if len(msg.ToolCalls()) > 0 {
				assistantMsg.ToolCalls = make([]openai.ChatCompletionMessageToolCallParam, len(msg.ToolCalls()))
				for i, call := range msg.ToolCalls() {
					assistantMsg.ToolCalls[i] = openai.ChatCompletionMessageToolCallParam{
						ID:   call.ID,
						Type: "function",
						Function: openai.ChatCompletionMessageToolCallFunctionParam{
							Name:      call.Name,
							Arguments: call.Input,
						},
					}
				}
			}

			openaiMessages = append(openaiMessages, openai.ChatCompletionMessageParamUnion{
				OfAssistant: &assistantMsg,
			})

		case message.Tool:
			for _, result := range msg.ToolResults() {
				openaiMessages = append(openaiMessages,
					openai.ToolMessage(result.Content, result.ToolCallID),
				)
			}
		}
	}

	return
}

func (o *openaiClient) convertTools(tools []tools.BaseTool) []openai.ChatCompletionToolParam {
	openaiTools := make([]openai.ChatCompletionToolParam, len(tools))

	for i, tool := range tools {
		info := tool.Info()
		openaiTools[i] = openai.ChatCompletionToolParam{
			Function: openai.FunctionDefinitionParam{
				Name:        info.Name,
				Description: openai.String(info.Description),
				Parameters: openai.FunctionParameters{
					"type":       "object",
					"properties": info.Parameters,
					"required":   info.Required,
				},
			},
		}
	}

	return openaiTools
}

func (o *openaiClient) finishReason(reason string) message.FinishReason {
	switch reason {
	case "stop":
		return message.FinishReasonEndTurn
	case "length":
		return message.FinishReasonMaxTokens
	case "tool_calls":
		return message.FinishReasonToolUse
	default:
		return message.FinishReasonUnknown
	}
}

func (o *openaiClient) preparedParams(messages []openai.ChatCompletionMessageParamUnion, tools []openai.ChatCompletionToolParam) openai.ChatCompletionNewParams {
	params := openai.ChatCompletionNewParams{
		Model:    openai.ChatModel(o.providerOptions.model.APIModel),
		Messages: messages,
		Tools:    tools,
	}

	if o.options.temperature != nil {
		params.Temperature = openai.Float(*o.options.temperature)
	}
	if o.options.topP != nil {
		params.TopP = openai.Float(*o.options.topP)
	}
	if o.options.seed != nil {
		params.Seed = openai.Int(*o.options.seed)
	}

	// Local OpenAI-compatible backends (for example Ollama) may reject reasoning fields.
	if o.providerOptions.model.Provider == models.ProviderLocal {
		params.MaxTokens = openai.Int(o.providerOptions.maxTokens)
		return params
	}

	if o.providerOptions.model.CanReason == true {
		params.MaxCompletionTokens = openai.Int(o.providerOptions.maxTokens)
		switch o.options.reasoningEffort {
		case "low":
			params.ReasoningEffort = shared.ReasoningEffortLow
		case "medium":
			params.ReasoningEffort = shared.ReasoningEffortMedium
		case "high":
			params.ReasoningEffort = shared.ReasoningEffortHigh
		default:
			params.ReasoningEffort = shared.ReasoningEffortMedium
		}
	} else {
		params.MaxTokens = openai.Int(o.providerOptions.maxTokens)
	}

	return params
}

func (o *openaiClient) requestOptions() []option.RequestOption {
	if o.providerOptions.model.Provider != models.ProviderLocal {
		return nil
	}

	requestOptions := []option.RequestOption{}
	if o.options.minP != nil {
		requestOptions = append(requestOptions,
			option.WithJSONSet("min_p", *o.options.minP),
			option.WithJSONSet("options.min_p", *o.options.minP),
		)
	}
	if o.options.topK != nil {
		requestOptions = append(requestOptions,
			option.WithJSONSet("top_k", *o.options.topK),
			option.WithJSONSet("options.top_k", *o.options.topK),
		)
	}
	if o.options.numCtx != nil {
		requestOptions = append(requestOptions,
			option.WithJSONSet("num_ctx", *o.options.numCtx),
			option.WithJSONSet("options.num_ctx", *o.options.numCtx),
		)
	}
	if o.options.repeatPenalty != nil {
		requestOptions = append(requestOptions,
			option.WithJSONSet("repeat_penalty", *o.options.repeatPenalty),
			option.WithJSONSet("options.repeat_penalty", *o.options.repeatPenalty),
		)
	}

	return requestOptions
}

func (o *openaiClient) send(ctx context.Context, messages []message.Message, tools []tools.BaseTool) (response *ProviderResponse, err error) {
	requestMessages := messages
	convertedMessages := o.convertMessages(requestMessages)
	convertedTools := o.convertTools(tools)
	if o.providerOptions.model.Provider == models.ProviderLocal && localModelHasNoTools(o.providerOptions.model) && len(convertedTools) > 0 {
		logging.WarnPersist(fmt.Sprintf("Model %s does not support tools; running without tools", o.providerOptions.model.APIModel))
		requestMessages = sanitizeMessagesForNoTools(requestMessages)
		convertedMessages = o.convertMessages(requestMessages)
		convertedTools = nil
	}
	params := o.preparedParams(convertedMessages, convertedTools)
	requestOptions := o.requestOptions()
	triedWithoutTools := false
	cfg := config.Get()
	if cfg.Debug {
		jsonData, _ := json.Marshal(params)
		logging.Debug("Prepared messages", "messages", string(jsonData))
	}
	attempts := 0
	for {
		attempts++
		openaiResponse, err := o.client.Chat.Completions.New(
			ctx,
			params,
			requestOptions...,
		)
		// If there is an error we are going to see if we can retry the call
		if err != nil {
			if o.providerOptions.model.Provider == models.ProviderLocal && len(convertedTools) > 0 && !triedWithoutTools && isUnsupportedToolsError(err) {
				markLocalModelNoTools(o.providerOptions.model)
				logging.WarnPersist(fmt.Sprintf("Model %s does not support tools; retrying without tools", o.providerOptions.model.APIModel))
				requestMessages = sanitizeMessagesForNoTools(messages)
				convertedMessages = o.convertMessages(requestMessages)
				params = o.preparedParams(convertedMessages, nil)
				triedWithoutTools = true
				continue
			}

			retry, after, retryErr := o.shouldRetry(attempts, err)
			if retryErr != nil {
				return nil, retryErr
			}
			if retry {
				logging.WarnPersist(fmt.Sprintf("Retrying due to rate limit... attempt %d of %d", attempts, maxRetries), logging.PersistTimeArg, time.Millisecond*time.Duration(after+100))
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(time.Duration(after) * time.Millisecond):
					continue
				}
			}
			return nil, retryErr
		}

		content := ""
		if openaiResponse.Choices[0].Message.Content != "" {
			content = openaiResponse.Choices[0].Message.Content
		}

		toolCalls := o.toolCalls(*openaiResponse)
		finishReason := o.finishReason(string(openaiResponse.Choices[0].FinishReason))

		if len(toolCalls) > 0 {
			finishReason = message.FinishReasonToolUse
		}

		return &ProviderResponse{
			Content:      content,
			ToolCalls:    toolCalls,
			Usage:        o.usage(*openaiResponse),
			FinishReason: finishReason,
		}, nil
	}
}

func (o *openaiClient) stream(ctx context.Context, messages []message.Message, tools []tools.BaseTool) <-chan ProviderEvent {
	requestMessages := messages
	convertedMessages := o.convertMessages(requestMessages)
	convertedTools := o.convertTools(tools)
	disabledByCache := false
	if o.providerOptions.model.Provider == models.ProviderLocal && localModelHasNoTools(o.providerOptions.model) && len(convertedTools) > 0 {
		disabledByCache = true
		requestMessages = sanitizeMessagesForNoTools(requestMessages)
		convertedMessages = o.convertMessages(requestMessages)
		convertedTools = nil
	}
	params := o.preparedParams(convertedMessages, convertedTools)
	params.StreamOptions = openai.ChatCompletionStreamOptionsParam{IncludeUsage: openai.Bool(true)}
	requestOptions := o.requestOptions()
	triedWithoutTools := false

	cfg := config.Get()
	if cfg.Debug {
		jsonData, _ := json.Marshal(params)
		logging.Debug("Prepared messages", "messages", string(jsonData))
	}

	attempts := 0
	eventChan := make(chan ProviderEvent)

	go func() {
		if disabledByCache {
			eventChan <- ProviderEvent{
				Type:    EventWarning,
				Content: fmt.Sprintf("Model %s does not support tools. Tools are disabled for this model.", o.providerOptions.model.APIModel),
			}
		}

		for {
			attempts++
			params.StreamOptions = openai.ChatCompletionStreamOptionsParam{IncludeUsage: openai.Bool(true)}
			openaiStream := o.client.Chat.Completions.NewStreaming(
				ctx,
				params,
				requestOptions...,
			)

			acc := openai.ChatCompletionAccumulator{}
			currentContent := ""
			toolCalls := make([]message.ToolCall, 0)

			for openaiStream.Next() {
				chunk := openaiStream.Current()
				acc.AddChunk(chunk)

				for _, choice := range chunk.Choices {
					if choice.Delta.Content != "" {
						eventChan <- ProviderEvent{
							Type:    EventContentDelta,
							Content: choice.Delta.Content,
						}
						currentContent += choice.Delta.Content
					}
				}
			}

			err := openaiStream.Err()
			if err == nil || errors.Is(err, io.EOF) {
				// Stream completed successfully
				finishReason := o.finishReason(string(acc.ChatCompletion.Choices[0].FinishReason))
				if len(acc.ChatCompletion.Choices[0].Message.ToolCalls) > 0 {
					toolCalls = append(toolCalls, o.toolCalls(acc.ChatCompletion)...)
				}
				if len(toolCalls) > 0 {
					finishReason = message.FinishReasonToolUse
				}

				eventChan <- ProviderEvent{
					Type: EventComplete,
					Response: &ProviderResponse{
						Content:      currentContent,
						ToolCalls:    toolCalls,
						Usage:        o.usage(acc.ChatCompletion),
						FinishReason: finishReason,
					},
				}
				close(eventChan)
				return
			}

			// If there is an error we are going to see if we can retry the call
			if o.providerOptions.model.Provider == models.ProviderLocal && len(convertedTools) > 0 && !triedWithoutTools && isUnsupportedToolsError(err) {
				markLocalModelNoTools(o.providerOptions.model)
				eventChan <- ProviderEvent{
					Type:    EventWarning,
					Content: fmt.Sprintf("Model %s does not support tools. Retrying without tools.", o.providerOptions.model.APIModel),
				}
				requestMessages = sanitizeMessagesForNoTools(messages)
				convertedMessages = o.convertMessages(requestMessages)
				params = o.preparedParams(convertedMessages, nil)
				triedWithoutTools = true
				continue
			}

			retry, after, retryErr := o.shouldRetry(attempts, err)
			if retryErr != nil {
				eventChan <- ProviderEvent{Type: EventError, Error: retryErr}
				close(eventChan)
				return
			}
			if retry {
				logging.WarnPersist(fmt.Sprintf("Retrying due to rate limit... attempt %d of %d", attempts, maxRetries), logging.PersistTimeArg, time.Millisecond*time.Duration(after+100))
				select {
				case <-ctx.Done():
					// context cancelled
					if ctx.Err() == nil {
						eventChan <- ProviderEvent{Type: EventError, Error: ctx.Err()}
					}
					close(eventChan)
					return
				case <-time.After(time.Duration(after) * time.Millisecond):
					continue
				}
			}
			eventChan <- ProviderEvent{Type: EventError, Error: retryErr}
			close(eventChan)
			return
		}
	}()

	return eventChan
}

func isUnsupportedToolsError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "does not support tools") ||
		strings.Contains(msg, "tool is not supported") ||
		strings.Contains(msg, "tools are not supported")
}

func localModelHasNoTools(model models.Model) bool {
	if model.Provider != models.ProviderLocal {
		return false
	}
	_, ok := localModelNoToolsCache.Load(localModelCapabilityKey(model))
	return ok
}

func markLocalModelNoTools(model models.Model) {
	if model.Provider != models.ProviderLocal {
		return
	}
	localModelNoToolsCache.Store(localModelCapabilityKey(model), true)
}

func localModelCapabilityKey(model models.Model) string {
	apiModel := strings.TrimSpace(model.APIModel)
	if apiModel != "" {
		return strings.ToLower(apiModel)
	}
	return strings.ToLower(string(model.ID))
}

func sanitizeMessagesForNoTools(messages []message.Message) []message.Message {
	out := make([]message.Message, 0, len(messages))
	for _, msg := range messages {
		if msg.Role == message.Tool {
			continue
		}

		filtered := msg
		filtered.Parts = make([]message.ContentPart, 0, len(msg.Parts))
		for _, part := range msg.Parts {
			switch part.(type) {
			case message.ToolCall, message.ToolResult:
				continue
			default:
				filtered.Parts = append(filtered.Parts, part)
			}
		}

		if filtered.Role == message.Assistant && strings.TrimSpace(filtered.Content().String()) == "" {
			continue
		}

		if filtered.Role == message.User && strings.TrimSpace(filtered.Content().String()) == "" && len(filtered.BinaryContent()) == 0 && len(filtered.ImageURLContent()) == 0 {
			continue
		}

		out = append(out, filtered)
	}
	return out
}

func (o *openaiClient) shouldRetry(attempts int, err error) (bool, int64, error) {
	var apierr *openai.Error
	if !errors.As(err, &apierr) {
		return false, 0, err
	}

	if apierr.StatusCode != 429 && apierr.StatusCode != 500 {
		return false, 0, err
	}

	if attempts > maxRetries {
		return false, 0, fmt.Errorf("maximum retry attempts reached for rate limit: %d retries", maxRetries)
	}

	retryMs := 0
	retryAfterValues := apierr.Response.Header.Values("Retry-After")

	backoffMs := 2000 * (1 << (attempts - 1))
	jitterMs := int(float64(backoffMs) * 0.2)
	retryMs = backoffMs + jitterMs
	if len(retryAfterValues) > 0 {
		if _, err := fmt.Sscanf(retryAfterValues[0], "%d", &retryMs); err == nil {
			retryMs = retryMs * 1000
		}
	}
	return true, int64(retryMs), nil
}

func (o *openaiClient) toolCalls(completion openai.ChatCompletion) []message.ToolCall {
	var toolCalls []message.ToolCall

	if len(completion.Choices) > 0 && len(completion.Choices[0].Message.ToolCalls) > 0 {
		for _, call := range completion.Choices[0].Message.ToolCalls {
			toolCall := message.ToolCall{
				ID:       call.ID,
				Name:     call.Function.Name,
				Input:    call.Function.Arguments,
				Type:     "function",
				Finished: true,
			}
			toolCalls = append(toolCalls, toolCall)
		}
	}

	return toolCalls
}

func (o *openaiClient) usage(completion openai.ChatCompletion) TokenUsage {
	cachedTokens := completion.Usage.PromptTokensDetails.CachedTokens
	inputTokens := completion.Usage.PromptTokens - cachedTokens

	return TokenUsage{
		InputTokens:         inputTokens,
		OutputTokens:        completion.Usage.CompletionTokens,
		CacheCreationTokens: 0, // OpenAI doesn't provide this directly
		CacheReadTokens:     cachedTokens,
	}
}

func WithOpenAIBaseURL(baseURL string) OpenAIOption {
	return func(options *openaiOptions) {
		options.baseURL = baseURL
	}
}

func WithOpenAIExtraHeaders(headers map[string]string) OpenAIOption {
	return func(options *openaiOptions) {
		options.extraHeaders = headers
	}
}

func WithOpenAIDisableCache() OpenAIOption {
	return func(options *openaiOptions) {
		options.disableCache = true
	}
}

func WithReasoningEffort(effort string) OpenAIOption {
	return func(options *openaiOptions) {
		defaultReasoningEffort := "medium"
		switch effort {
		case "low", "medium", "high":
			defaultReasoningEffort = effort
		default:
			logging.Warn("Invalid reasoning effort, using default: medium")
		}
		options.reasoningEffort = defaultReasoningEffort
	}
}

func WithOpenAITemperature(value *float64) OpenAIOption {
	return func(options *openaiOptions) {
		options.temperature = value
	}
}

func WithOpenAITopP(value *float64) OpenAIOption {
	return func(options *openaiOptions) {
		options.topP = value
	}
}

func WithOpenAIMinP(value *float64) OpenAIOption {
	return func(options *openaiOptions) {
		options.minP = value
	}
}

func WithOpenAITopK(value *int64) OpenAIOption {
	return func(options *openaiOptions) {
		options.topK = value
	}
}

func WithOpenAINumCtx(value *int64) OpenAIOption {
	return func(options *openaiOptions) {
		options.numCtx = value
	}
}

func WithOpenAIRepeatPenalty(value *float64) OpenAIOption {
	return func(options *openaiOptions) {
		options.repeatPenalty = value
	}
}

func WithOpenAISeed(value *int64) OpenAIOption {
	return func(options *openaiOptions) {
		options.seed = value
	}
}
