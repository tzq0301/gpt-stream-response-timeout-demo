package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/samber/lo"
	"github.com/spf13/viper"
)

const (
	configFile = ".env"

	configKeyURLPrefix    = "URL_PREFIX"
	configKeyOpenAIAPIKey = "OPENAI_API_KEY"
)

var (
	urlPrefix    string
	openaiAPIKey string
)

func init() {
	viper.SetConfigFile(configFile)
	lo.Must0(viper.ReadInConfig())
	urlPrefix = viper.GetString(configKeyURLPrefix)
	openaiAPIKey = viper.GetString(configKeyOpenAIAPIKey)
}

func main() {
	// not stream

	ctx := context.TODO()

	response := httpRequestGPT(ctx)

	println(string(lo.Must(io.ReadAll(response.Body))))
}

func httpRequestGPT(ctx context.Context) *http.Response {
	request := lo.Must(http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/chat/completions", urlPrefix), bytes.NewBuffer(lo.Must(json.Marshal(map[string]any{
		"model": "gpt-3.5-turbo",
		"messages": []any{
			map[string]any{
				"role":    "system",
				"content": "You are a helpful assistant.",
			},
			map[string]any{
				"role":    "user",
				"content": "Hello!",
			},
		},
	})))))

	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", fmt.Sprintf("Bearer %s", openaiAPIKey))

	response := lo.Must((&http.Client{}).Do(request))

	return response
}
