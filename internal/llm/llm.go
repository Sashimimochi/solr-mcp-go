package llm

import (
    "bytes"
    "context"
    "encoding/json"
    "errors"
    "fmt"
    "io"
    "log/slog"
    "net/http"
    "strings"

    "solr-mcp-go/internal/types"
)

type LLMConfig struct {
    HttpClient *http.Client
    BaseURL    string
    APIKey     string
    Model      string
}

type EmbeddingConfig struct {
    HttpClient *http.Client
    BaseURL    string
    APIKey     string
    Model      string
}

func CallLLMForPlan(ctx context.Context, cfg LLMConfig, userQuery, locale, schemaSummary string, allowVector, allowHybrid bool) (*types.LlmPlan, map[string]any, error) {
    sys := `You are a Solr search planner. Output a STRICT JSON object only (no prose).
Map the user's natural language to Solr query params. Consider schema and guesses.
Prefer eDisMax for textual relevance. If vector search is allowed, you may choose "vector" or "hybrid".
For "ranges", output ISO-8601 dates for date fields. Do not invent fields that do not exist.`

    user := fmt.Sprintf(`locale=%s
SCHEMA:
%s
CAPABILITIES:
allowVector=%v
allowHybrid=%v
GOAL:
Given the user query, produce a plan:
- mode: "keyword", "vector", or "hybrid"
- edismax: { text_query, filters[], ranges[], sort, facet_fields[], params{qf,pf,mm,tie,df,...}, fields[] }
- vector: {field, k, query_text}
- rows/start (optional)

USER QUERY:
%s`, locale, schemaSummary, allowVector, allowHybrid, userQuery)

    req := map[string]any{
        "model": cfg.Model,
        "messages": []map[string]string{
            {"role": "system", "content": sys},
            {"role": "user", "content": user},
        },
        "temperature":     0.2,
        "response_format": map[string]string{"type": "json_object"},
        "max_tokens":      800,
    }
    url := cfg.BaseURL + "/chat/completions"
    slog.Debug("Calling LLM for plan", "url", url, "user_prompt", user)
    out, err := post(ctx, cfg.HttpClient, url, req, cfg.APIKey)
    if err != nil {
        return nil, nil, err
    }
    content := getFirstChoiceContent(out)
    if strings.TrimSpace(content) == "" {
        return nil, nil, errors.New("LLM returned empty content")
    }
    var plan types.LlmPlan
    if err := json.Unmarshal([]byte(content), &plan); err != nil {
        return nil, nil, fmt.Errorf("failed to parse LLM response as JSON: %v\nresponse was: %s", err, content)
    }
    var planMap map[string]any
    _ = json.Unmarshal([]byte(content), &planMap)
    if plan.Mode == "" {
        plan.Mode = "keyword"
    }
    if plan.EdisMax.TextQuery == "" {
        plan.EdisMax.TextQuery = "*:*"
    }
    if plan.Vector.K == 0 {
        plan.Vector.K = 100
    }
    return &plan, planMap, nil
}

func EnsureEmbedding(ctx context.Context, cfg EmbeddingConfig, text string) ([]float64, error) {
    if cfg.BaseURL == "" || cfg.APIKey == "" {
        return nil, errors.New("EMBEDDING_BASE_URL and EMBEDDING_API_KEY must be set for vector search")
    }
    req := map[string]any{
        "model": cfg.Model,
        "input": text,
    }
    out, err := post(ctx, cfg.HttpClient, cfg.BaseURL, req, cfg.APIKey)
    if err != nil {
        return nil, err
    }
    data, _ := out["data"].([]any)
    if len(data) == 0 {
        return nil, errors.New("embedding API returned no data")
    }
    item, _ := data[0].(map[string]any)
    vecAny, _ := item["embedding"].([]any)
    vec := make([]float64, 0, len(vecAny))
    for _, v := range vecAny {
        if f, ok := v.(float64); ok {
            vec = append(vec, f)
        }
    }
    if len(vec) == 0 {
        return nil, errors.New("empty embedding vector")
    }
    return vec, nil
}

func post(ctx context.Context, httpClient *http.Client, url string, body any, apiKey string) (map[string]any, error) {
    var r io.Reader
    buf, _ := json.Marshal(body)
    r = bytes.NewReader(buf)

    req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, r)
    if err != nil {
        return nil, fmt.Errorf("failed to create HTTP request: %v", err)
    }
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Authorization", "Bearer "+apiKey)
    res, err := httpClient.Do(req)
    if err != nil {
        return nil, fmt.Errorf("HTTP request error: %v", err)
    }
    defer res.Body.Close()

    bodyBytes, err := io.ReadAll(res.Body)
    if err != nil {
        return nil, fmt.Errorf("failed to read response body: %v", err)
    }

    var out map[string]any
    if err := json.Unmarshal(bodyBytes, &out); err == nil {
        return out, nil
    }

    var outArr []map[string]any
    if err := json.Unmarshal(bodyBytes, &outArr); err == nil {
        if len(outArr) > 0 {
            return outArr[0], nil
        }
        return nil, errors.New("LLM returned an empty array")
    }

    return nil, fmt.Errorf("JSON decode error: %v. Response: %s", err, string(bodyBytes))
}

func getFirstChoiceContent(m map[string]any) string {
    if errVal, ok := m["error"]; ok {
        slog.Error("LLM API returned an error: %v", errVal)
        return ""
    }
    choicesVal, ok := m["choices"]
    if !ok || choicesVal == nil {
        return ""
    }
    choices, ok := choicesVal.([]any)
    if !ok || len(choices) == 0 {
        return ""
    }
    ch0, ok := choices[0].(map[string]any)
    if !ok {
        return ""
    }
    msg, ok := ch0["message"].(map[string]any)
    if !ok {
        return ""
    }
    content, _ := msg["content"].(string)
    return content
}