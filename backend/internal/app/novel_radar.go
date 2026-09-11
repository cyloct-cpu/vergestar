package app

import (
	"context"
)

// T4：市场雷达（runRadar）与互动影游确认动作。

type NovelRadarRequest struct {
	BaseUrl    string  `json:"baseUrl"`
	ApiKey     string  `json:"apiKey"`
	Model      string  `json:"model"`
	ApiFormat  string  `json:"apiFormat,omitempty"`
	Temperature float64 `json:"temperature,omitempty"`
}

// NovelMarketRadar runs the InkOS RadarAgent trend scan with the caller's model config.
func (s *Service) NovelMarketRadar(ctx context.Context, userID string, request NovelRadarRequest) (map[string]any, error) {
	if request.BaseUrl == "" || request.ApiKey == "" || request.Model == "" {
		return nil, BadAuthRequest("市场雷达需要模型配置")
	}
	payload := map[string]any{
		"userId": userID,
		"model": map[string]any{
			"baseUrl":     request.BaseUrl,
			"apiKey":      request.ApiKey,
			"model":       request.Model,
			"apiFormat":   request.ApiFormat,
			"temperature": request.Temperature,
		},
	}
	var body struct {
		Result map[string]any `json:"result"`
	}
	if err := s.novelBridgeCall(ctx, userID, "/agent/books/radar", payload, &body); err != nil {
		return nil, err
	}
	return body.Result, nil
}
