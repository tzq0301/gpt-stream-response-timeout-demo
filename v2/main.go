package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	jsoniter "github.com/json-iterator/go"
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
	// stream response

	ctx := context.TODO()

	response := httpRequestGPT(ctx)

	scanner := bufio.NewScanner(response.Body)

	{
		firstLine := []byte(trimPrefix(readOneLine(scanner)))

		messageID := jsoniter.Get(firstLine, "id").ToString()
		model := jsoniter.Get(firstLine, "model").ToString()

		fmt.Println()
		fmt.Printf("MessageID = %s\n", messageID)
		fmt.Printf("Model     = %s\n", model)
	}

	fmt.Printf("Content   = ")

	for scanner.Scan() {
		line := trimPrefix(scanner.Text())

		if isEmpty(line) {
			continue
		}

		if isFinal(line) {
			_ = readOneLine(scanner)
			break
		}

		fmt.Print(jsoniter.Get([]byte(line), "choices", 0, "delta", "content").ToString())
	}

	fmt.Println()
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
		"stream": true,
	})))))

	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", fmt.Sprintf("Bearer %s", openaiAPIKey))

	response := lo.Must((&http.Client{}).Do(request))

	return response
}

func isEmpty(line string) bool {
	return line == ""
}

func trimPrefix(line string) string {
	return strings.TrimPrefix(line, "data: ")
}

func isFinal(line string) bool {
	return jsoniter.Get([]byte(line), "choices", 0, "finish_reason").ToString() == "stop"
}

func readOneLine(scanner *bufio.Scanner) string {
	if scanner.Scan() {
		return scanner.Text()
	}

	panic(errors.New("readOneLine"))
}
