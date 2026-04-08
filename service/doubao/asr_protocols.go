package doubao

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/bytedance/sonic"
	"github.com/google/uuid"
	"google.golang.org/grpc/grpclog"
)

type UserMeta struct {
	Uid        string `json:"uid,omitempty"`
	Did        string `json:"did,omitempty"`
	Platform   string `json:"platform,omitempty" `
	SDKVersion string `json:"sdk_version,omitempty"`
	APPVersion string `json:"app_version,omitempty"`
}

type AudioMeta struct {
	Format   string `json:"format,omitempty"`
	Codec    string `json:"codec,omitempty"`
	Rate     int    `json:"rate,omitempty"`
	Bits     int    `json:"bits,omitempty"`
	Channel  int    `json:"channel,omitempty"`
	URL      string `json:"url,omitempty"`
	Language string `json:"language,omitempty"`
}

type CorpusMeta struct {
	BoostingTableName string `json:"boosting_table_name,omitempty"`
	CorrectTableName  string `json:"correct_table_name,omitempty"`
	Context           string `json:"context,omitempty"`
}

type RequestMeta struct {
	ModelName      string     `json:"model_name,omitempty"`
	EnableITN      bool       `json:"enable_itn,omitempty"`
	EnablePUNC     bool       `json:"enable_punc,omitempty"`
	EnableDDC      bool       `json:"enable_ddc,omitempty"`
	ShowUtterances bool       `json:"show_utterancies,omitempty"`
	Corpus         CorpusMeta `json:"corpus,omitempty"`
}

type AsrRequestPayload struct {
	User    UserMeta    `json:"user"`
	Audio   AudioMeta   `json:"audio"`
	Request RequestMeta `json:"request"`
}

type AsrResponse struct {
	Result struct {
		Text       string `json:"text"`
		Utterances []struct {
			Definite  bool   `json:"definite"`
			EndTime   int    `json:"end_time"`
			StartTime int    `json:"start_time"`
			Text      string `json:"text"`
			Words     []struct {
				EndTime   int    `json:"end_time"`
				StartTime int    `json:"start_time"`
				Text      string `json:"text"`
			} `json:"words"`
		} `json:"utterances"`
	} `json:"result"`
}

func DefaultPayload(fileURL string) *AsrRequestPayload {
	return &AsrRequestPayload{
		User: UserMeta{
			Uid: "demo_uid",
		},
		Audio: AudioMeta{
			Format:   "mp3",
			Codec:    "raw",
			Rate:     16000,
			Bits:     16,
			Channel:  1,
			URL:      fileURL,
			Language: "en-US",
		},
		Request: RequestMeta{
			ModelName:  "bigmodel",
			EnableITN:  true,
			EnablePUNC: true,
			EnableDDC:  true,
		},
	}
}

type AsrHttpClient struct {
	url string
}

func NewAsrHttpClient(url string) *AsrHttpClient {
	return &AsrHttpClient{
		url: url,
	}
}

func (c *AsrHttpClient) submit(ctx context.Context, reqID string, fileUrl string) (string, error) {
	submitUrl := c.url + "/submit"
	header := NewAuthHeader(reqID, "volc.bigasr.auc")
	payload := DefaultPayload(fileUrl)

	payloadData, err := sonic.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal payload: %w", err)
	}
	submitRequest, err := http.NewRequest(http.MethodPost, submitUrl, bytes.NewBuffer(payloadData))
	if err != nil {
		return "", fmt.Errorf("failed to create submit request: %w", err)
	}
	submitRequest.Header = header
	submitRequest.Header.Set("Content-Type", "application/json")
	submitRequest.WithContext(ctx)
	// 使用HTTP客户端发送请求
	client := &http.Client{}
	resp, err := client.Do(submitRequest)
	if err != nil {
		return "", fmt.Errorf("failed to do request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("response failed, status code: %d", resp.StatusCode)
	}
	statusCode := resp.Header.Get("X-Api-Status-Code")
	message := resp.Header.Get("X-Api-Message")
	logID := resp.Header.Get("X-Tt-Logid")
	if statusCode != "20000000" {
		return "", fmt.Errorf("response failed, status code: %s, message: %s", statusCode, message)
	}

	return logID, nil
}

func (c *AsrHttpClient) doQuery(ctx context.Context, reqID string) ([]byte, http.Header, error) {
	queryUrl := c.url + "/query"
	header := NewAuthHeader(reqID, "volc.bigasr.auc")
	queryRequest, err := http.NewRequest(http.MethodPost, queryUrl, bytes.NewBuffer([]byte("{}")))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create query request: %w", err)
	}
	queryRequest.Header = header
	queryRequest.Header.Set("Content-Type", "application/json")
	queryRequest.WithContext(ctx)
	// 使用HTTP客户端发送请求
	client := &http.Client{}
	resp, err := client.Do(queryRequest)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to do request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("response failed, status code: %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read response body: %w", err)
	}
	return body, resp.Header, nil
}

func (c *AsrHttpClient) query(ctx context.Context, reqID string) (*AsrResponse, error) {
	for {
		body, header, err := c.doQuery(ctx, reqID)
		if err != nil {
			return nil, fmt.Errorf("failed to do query: %w", err)
		}
		code := header.Get("X-Api-Status-Code")
		message := header.Get("X-Api-Message")
		if code == "20000000" {
			var resp AsrResponse
			if err := sonic.Unmarshal(body, &resp); err != nil {
				return nil, fmt.Errorf("failed to unmarshal response: %w", err)
			}
			return &resp, nil
		}
		if code != "20000001" && code != "20000002" {
			return nil, fmt.Errorf("response failed, status code: %s, message: %s", code, message)
		}
		time.Sleep(time.Second * 3)
	}
}

func (c *AsrHttpClient) Excute(ctx context.Context, fileURL string) (*AsrResponse, error) {
	if c.url == "" {
		return nil, errors.New("url is empty")
	}
	// reqID 代表一个任务
	reqID := uuid.New().String()
	// submit，logID 代表一个请求
	logID, err := c.submit(ctx, reqID, fileURL)
	if err != nil {
		return nil, fmt.Errorf("failed to submit request: %w", err)
	}
	grpclog.Infof("task submitted, logID: %s", logID)
	// for loop do query
	resp, err := c.query(ctx, reqID)
	if err != nil {
		return nil, fmt.Errorf("task failed: %w", err)
	}

	return resp, nil
}
