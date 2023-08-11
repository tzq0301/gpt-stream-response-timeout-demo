package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	jsoniter "github.com/json-iterator/go"
	"github.com/samber/lo"
	"github.com/spf13/viper"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

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
	messageID, model, respCh := requestGPT()

	fmt.Println()
	fmt.Printf("MessageID = %s\n", messageID)
	fmt.Printf("Model     = %s\n", model)
	fmt.Printf("Content   = ")

	func() {
		for {
			select {
			case resp, ok := <-respCh:
				if !ok {
					return // channel has been closed
				}
				fmt.Print(resp)
			default:
			}
		}
	}()

	fmt.Println()
}

func requestGPT() (messageID string, model string, respCh chan string) {
	// duration for killing the goroutine for avoiding 'goroutine leak'
	// duration should be set 99% larger than the duration of real chat in order to not end the chat
	ctx, cancel := context.WithTimeout(context.TODO(), 2*time.Minute)

	ch := make(chan *http.Response, 1)

	go func() {
		ch <- httpRequestGPT(ctx)
	}()

	var response *http.Response

	select {
	// duration for achieving the http response (status code, etc.)
	// if status code can not be got within the duration, then we know current http request is timeout
	case <-time.After(time.Second * 10):
		panic(errors.New("timeout"))
	// got response!!!
	case response = <-ch:
		log.Printf("got response, status code = %v", response.StatusCode)
	}

	if response == nil {
		panic(errors.New("request GPT timeout"))
	}

	log.Println("already got the response, start to handle")

	scanner := bufio.NewScanner(response.Body)

	{
		// handle the first line with block, get the meta information (such as message id, model, etc.)
		// then return the function quickly, the stream response will be returned by the channel

		firstLine := []byte(trimPrefix(readOneLine(scanner)))

		messageID = jsoniter.Get(firstLine, "id").ToString()
		model = jsoniter.Get(firstLine, "model").ToString()
	}

	respCh = make(chan string)

	go func() {
		// the timeout of function httpRequestGPT handled does not mean the timeout of httpClient.Do is handled,
		// the http request is continually hanging, which mean the goroutine is leaked, thus we should cancel the http request
		// to avoid the goroutine being leaked
		defer cancel()

		// close the channel when finish
		defer close(respCh)

		for scanner.Scan() {
			line := trimPrefix(scanner.Text())

			if isEmpty(line) {
				continue
			}

			if isFinal(line) {
				_ = readOneLine(scanner)
				break
			}

			respCh <- jsoniter.Get([]byte(line), "choices", 0, "delta", "content").ToString()
		}
	}()

	return
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
