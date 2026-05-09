package handler

import "github.com/Wei-Shaw/sub2api/internal/service"

func convertOpenAIForwardResultToForwardResult(result *service.OpenAIForwardResult) *service.ForwardResult {
	if result == nil {
		return nil
	}
	return &service.ForwardResult{
		RequestID: result.RequestID,
		Usage: service.ClaudeUsage{
			InputTokens:              result.Usage.InputTokens,
			OutputTokens:             result.Usage.OutputTokens,
			CacheCreationInputTokens: result.Usage.CacheCreationInputTokens,
			CacheReadInputTokens:     result.Usage.CacheReadInputTokens,
			ImageOutputTokens:        result.Usage.ImageOutputTokens,
		},
		Model:           result.Model,
		UpstreamModel:   result.UpstreamModel,
		Stream:          result.Stream,
		Duration:        result.Duration,
		FirstTokenMs:    result.FirstTokenMs,
		ReasoningEffort: result.ReasoningEffort,
		ImageCount:      result.ImageCount,
		ImageSize:       result.ImageSize,
	}
}
