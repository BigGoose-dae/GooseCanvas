package provider

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/BigGoose-dae/GooseCanvas/internal/config"
)

type AssetLibraryItem struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	GroupID           string `json:"groupId"`
	AssetType         string `json:"assetType"`
	Status            string `json:"status"`
	ProjectName       string `json:"projectName"`
	CreateTime        string `json:"createTime,omitempty"`
	UpdateTime        string `json:"updateTime,omitempty"`
	LastInferenceTime string `json:"lastInferenceTime,omitempty"`
	Error             any    `json:"error,omitempty"`
}

type AssetLibraryPage struct {
	Items      []AssetLibraryItem `json:"items"`
	TotalCount int                `json:"totalCount"`
	PageNumber int                `json:"pageNumber"`
	PageSize   int                `json:"pageSize"`
}

type AssetsClient struct {
	cfg  config.Volcengine
	http *http.Client
	now  func() time.Time
}

func NewAssetsClient(cfg config.Volcengine) *AssetsClient {
	return &AssetsClient{cfg: cfg, http: &http.Client{Timeout: 30 * time.Second}, now: time.Now}
}

func (c *AssetsClient) Configured() bool {
	return strings.TrimSpace(c.cfg.AssetsAccessKey) != "" && strings.TrimSpace(c.cfg.AssetsSecretKey) != ""
}

func (c *AssetsClient) List(ctx context.Context, page, pageSize int) (AssetLibraryPage, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 100
	}
	payload := map[string]any{
		"PageNumber": page, "PageSize": pageSize,
		"SortBy": "CreateTime", "SortOrder": "Desc",
		"ProjectName": strings.TrimSpace(c.cfg.AssetsProjectName),
		"Filter":      map[string]any{"GroupType": "AIGC"},
	}
	var raw struct {
		Items []struct {
			ID                string `json:"Id"`
			Name              string `json:"Name"`
			GroupID           string `json:"GroupId"`
			AssetType         string `json:"AssetType"`
			Status            string `json:"Status"`
			ProjectName       string `json:"ProjectName"`
			CreateTime        string `json:"CreateTime"`
			UpdateTime        string `json:"UpdateTime"`
			LastInferenceTime string `json:"LastInferenceTime"`
			Error             any    `json:"Error"`
		} `json:"Items"`
		TotalCount int `json:"TotalCount"`
		PageNumber int `json:"PageNumber"`
		PageSize   int `json:"PageSize"`
	}
	if err := c.do(ctx, "ListAssets", payload, &raw); err != nil {
		return AssetLibraryPage{}, err
	}
	result := AssetLibraryPage{TotalCount: raw.TotalCount, PageNumber: raw.PageNumber, PageSize: raw.PageSize, Items: make([]AssetLibraryItem, 0, len(raw.Items))}
	for _, item := range raw.Items {
		result.Items = append(result.Items, AssetLibraryItem{ID: item.ID, Name: item.Name, GroupID: item.GroupID, AssetType: item.AssetType, Status: item.Status, ProjectName: item.ProjectName, CreateTime: item.CreateTime, UpdateTime: item.UpdateTime, LastInferenceTime: item.LastInferenceTime, Error: item.Error})
	}
	return result, nil
}

func (c *AssetsClient) do(ctx context.Context, action string, payload, target any) error {
	if !c.Configured() {
		return fmt.Errorf("火山素材库 IAM AK/SK 未配置")
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	base := strings.TrimRight(strings.TrimSpace(c.cfg.AssetsBaseURL), "/")
	if base == "" {
		base = config.DefaultAssetsBaseURL
	}
	endpoint, err := url.Parse(base)
	if err != nil {
		return err
	}
	endpoint.Path = "/"
	query := url.Values{"Action": {action}, "Version": {"2024-01-01"}}
	endpoint.RawQuery = canonicalAssetQuery(query)
	now := c.now().UTC()
	xDate := now.Format("20060102T150405Z")
	shortDate := now.Format("20060102")
	contentHash := sha256String(body)
	signedHeaders := "content-type;host;x-content-sha256;x-date"
	canonicalHeaders := "content-type:application/json\nhost:" + endpoint.Host + "\nx-content-sha256:" + contentHash + "\nx-date:" + xDate + "\n"
	canonicalRequest := http.MethodPost + "\n/\n" + endpoint.RawQuery + "\n" + canonicalHeaders + "\n" + signedHeaders + "\n" + contentHash
	scope := shortDate + "/cn-beijing/ark/request"
	stringToSign := "HMAC-SHA256\n" + xDate + "\n" + scope + "\n" + sha256String([]byte(canonicalRequest))
	signingKey := hmacBytes(hmacBytes(hmacBytes(hmacBytes([]byte(strings.TrimSpace(c.cfg.AssetsSecretKey)), shortDate), "cn-beijing"), "ark"), "request")
	signature := fmt.Sprintf("%x", hmacBytes(signingKey, stringToSign))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Date", xDate)
	req.Header.Set("X-Content-Sha256", contentHash)
	req.Header.Set("Authorization", "HMAC-SHA256 Credential="+strings.TrimSpace(c.cfg.AssetsAccessKey)+"/"+scope+", SignedHeaders="+signedHeaders+", Signature="+signature)
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("连接火山素材库失败: %w", err)
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return err
	}
	var envelope struct {
		ResponseMetadata struct {
			RequestID string `json:"RequestId"`
			Error     *struct {
				Code    string `json:"Code"`
				Message string `json:"Message"`
			} `json:"Error"`
		} `json:"ResponseMetadata"`
		Result json.RawMessage `json:"Result"`
	}
	if len(responseBody) > 0 && json.Unmarshal(responseBody, &envelope) != nil {
		return fmt.Errorf("火山素材库返回了无法解析的响应（HTTP %d）", resp.StatusCode)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 || envelope.ResponseMetadata.Error != nil {
		message := strings.TrimSpace(string(responseBody))
		code := ""
		if envelope.ResponseMetadata.Error != nil {
			code = envelope.ResponseMetadata.Error.Code
			message = envelope.ResponseMetadata.Error.Message
		}
		if code != "" {
			message = code + ": " + message
		}
		if envelope.ResponseMetadata.RequestID != "" {
			message += "（RequestId: " + envelope.ResponseMetadata.RequestID + "）"
		}
		return &HTTPError{Status: resp.StatusCode, Message: message}
	}
	if target == nil {
		return nil
	}
	raw := responseBody
	if len(envelope.Result) > 0 && string(envelope.Result) != "null" {
		raw = envelope.Result
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return fmt.Errorf("解析火山素材库响应: %w", err)
	}
	return nil
}

func canonicalAssetQuery(values url.Values) string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		for _, value := range values[key] {
			parts = append(parts, url.QueryEscape(key)+"="+url.QueryEscape(value))
		}
	}
	return strings.ReplaceAll(strings.Join(parts, "&"), "+", "%20")
}
func sha256String(data []byte) string { sum := sha256.Sum256(data); return fmt.Sprintf("%x", sum[:]) }
func hmacBytes(key []byte, value string) []byte {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(value))
	return mac.Sum(nil)
}
