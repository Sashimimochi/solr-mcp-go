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
	"time"

	"solr-mcp-go/internal/types"
	"solr-mcp-go/internal/utils"
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
	timezone := utils.GetTimezone()
	sys := `You are a Solr query translator for non-technical users.
Users know NOTHING about Solr, schemas, or query syntax.

YOUR JOB:
1. Interpret vague, ambiguous natural language
2. Map concepts to available schema fields using fuzzy matching
3. Make reasonable assumptions and document them
4. Generate working Solr queries even from terrible inputs

OUTPUT: JSON only (no prose).`

	user := fmt.Sprintf(`CONTEXT:
- Current time: %s (timezone: %s)
- Locale: %s

USER PROFILE:
- Does NOT know field names, types, or Solr syntax
- Uses colloquial terms (e.g., "crashed", "broken", "yesterday night")
- Often omits crucial details
- May reference concepts not in schema

SCHEMA (with semantic hints):
%s

USER QUERY:
"%s"

TRANSLATION TASK:

PHASE 1: INTENT ANALYSIS
Decompose the query into:
- Temporal: when? (relative times like "yesterday", "last hour", "this morning")
- Entities: what things? (services, components, users, IDs)
- Conditions: what states? (errors, failures, success, specific values)
- Actions: what result? (find, count, show details, summarize)
- Context clues: implicit filters (e.g., "database crashed" implies error level)

PHASE 2: FIELD MAPPING
For each extracted concept, find best matching field(s):

Strategies:
- **Exact match**: User says "component:api" → use as-is
- **Semantic match**: "database" → check fields with types like 'service', 'component', 'application'
- **Content search**: If no field match, search in text fields (message, description, log)
- **Multiple candidates**: Use OR across possible fields (e.g., component:database OR message:database)
- **Missing mapping**: Document assumption in _reasoning

Time expressions:
- "yesterday" → [2025-10-02T00:00:00Z TO 2025-10-02T23:59:59Z] (full day in user's timezone)
- "last night" → assume 18:00-06:00 previous day
- "this morning" → 06:00-12:00 today
- "last hour" → [NOW-1HOUR TO NOW]
- Handle ambiguity: "last week" = past 7 days or previous calendar week?

Colloquial terms:
- "crashed", "down", "died", "broken" → level:ERROR OR status:DOWN OR message:(*crash* OR *fail* OR *down*)
- "slow", "hanging", "stuck" → response_time:>5000 OR message:(*timeout* OR *slow*)
- "weird", "strange", "odd" → level:WARN OR level:ERROR
- "all hell broke loose" → level:CRITICAL OR level:ERROR with high frequency

PHASE 3: QUERY CONSTRUCTION

Build Solr params:
- **q**: Textual search terms (ranked relevance)
- **fq**: Filters that are certain (time ranges, exact matches, levels)
- **defType**: Usually "edismax" for flexible text search
- **qf**: Weight fields by likelihood (message^1.5 stack_trace^2 title^3)
- **mm**: Set minimum match (e.g., "75%%" for fuzzy matching)
- **facet.field**: Auto-suggest useful facets (level, component, user)
- **fl**: Return relevant fields based on action intent
- **rows**: Default 20-50 for "show me", 0 for "how many"
- **sort**: Default by relevance, or timestamp:desc if time-focused
- **hl**: Enable if user wants to "see" or "show" text content

PHASE 4: ASSUMPTIONS & FALLBACKS

Document decisions in _reasoning:
- Field mappings that are uncertain
- Default values used (time ranges, match thresholds)
- Ignored terms (unmappable words)
- Suggested alternative queries

If critical info is missing:
- Default to recent time range (last 24h) unless specified
- Include common filters (e.g., exclude test environments)
- Bias toward ERROR level if problem-focused language

PHASE 5: QUALITY CHECKS

Ensure query will work:
- All field names exist in schema
- Date ranges are valid ISO-8601
- No syntax errors in fq
- At least one of: q has content, or fq has filters
- If query is empty, default to recent entries: fq:timestamp:[NOW-24HOUR TO NOW]

OUTPUT FORMAT:
{
  "params": {
    "q": "...",
    "fq": ["...", "..."],
    "defType": "edismax",
    "qf": "field1^2 field2^1",
    "mm": "75%%",
    "facet": true,
    "facet.field": ["level", "component"],
    "fl": "timestamp,level,component,message,id",
    "rows": 20,
    "sort": "timestamp desc",
    "hl": true,
    "hl.fl": "message"
  },
  "_reasoning": {
    "temporal_interpretation": "...",
    "field_mappings": {
      "database": ["component:database", "message:database"],
      "crashed": ["level:ERROR", "message:(*crash* OR *fail*)"]
    },
    "assumptions": ["...", "..."],
    "confidence": "high|medium|low",
    "alternative_queries": ["..."]
  }
}

EXAMPLES:

Input: "昨日の夜データベースが落ちた時のやつ見せて"
Output:
{
  "params": {
    "q": "(*crash* OR *fail* OR *down* OR *error*)",
    "fq": [
      "timestamp:[2025-10-02T18:00:00%s TO 2025-10-03T06:00:00%s]",
      "(component:database OR service:database OR message:database)",
      "(level:ERROR OR level:CRITICAL)"
    ],
    "defType": "edismax",
    "qf": "message^2 stack_trace^1.5 error_message^3",
    "mm": "1",
    "facet": true,
    "facet.field": ["level", "component", "error_code"],
    "fl": "timestamp,level,component,message,stack_trace,id",
    "rows": 50,
    "sort": "timestamp desc",
    "hl": true,
    "hl.fl": "message,stack_trace"
  },
  "_reasoning": {
    "temporal_interpretation": "昨日の夜 = 2025-10-02 18:00 to 2025-10-03 06:00",
    "field_mappings": {
      "データベース": "Mapped to component:database (primary) + message:database (fallback)",
      "落ちた": "Mapped to level:ERROR + crash/fail/down keywords in message"
    },
    "assumptions": [
      "夜 = 18:00-06:00 based on common definition",
      "見せて = show details, so returned multiple fields and highlighting",
      "Added level:CRITICAL as alternative to ERROR"
    ],
    "confidence": "high",
    "alternative_queries": [
      "If no results: expand time range to full day",
      "If too many results: add fq:severity:HIGH"
    ]
  }
}

Input: "APIが重い気がする"
Output:
{
  "params": {
    "q": "(*slow* OR *timeout* OR *latency* OR *performance*)",
    "fq": [
      "timestamp:[NOW-24HOUR TO NOW]",
      "(component:api OR service:api OR endpoint:*)",
      "(level:WARN OR level:ERROR OR response_time:[5000 TO *])"
    ],
    "defType": "edismax",
    "qf": "message^2 description^1.5",
    "mm": "1",
    "stats": true,
    "stats.field": "response_time",
    "facet": true,
    "facet.field": ["endpoint", "level"],
    "fl": "timestamp,endpoint,response_time,level,message",
    "rows": 30,
    "sort": "response_time desc"
  },
  "_reasoning": {
    "temporal_interpretation": "気がする (feeling) = recent issue, default to last 24h",
    "field_mappings": {
      "API": "component:api OR service:api",
      "重い": "response_time:>5000ms OR keywords: slow/timeout/latency"
    },
    "assumptions": [
      "重い = high response time, set threshold at 5000ms",
      "Added stats.field to show response_time distribution",
      "Included WARN level as performance issues may not be ERROR"
    ],
    "confidence": "medium",
    "alternative_queries": [
      "If response_time field doesn't exist, rely on text search only",
      "Consider faceting by time ranges to see when it got slow"
    ]
  }
}

CRITICAL REMINDERS:
- Users are NON-TECHNICAL - translate everything
- Ambiguity is NORMAL - make best guess and document
- Query must WORK even if suboptimal
- ALWAYS include _reasoning for debugging
- Bias toward returning SOMETHING over returning nothing`,
		time.Now().Format(time.RFC3339),
		timezone,
		locale,
		schemaSummary,
		userQuery,
		timezone, timezone)

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
		plan.Vector.K = 5
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
		slog.Error("LLM API returned an error", "error", errVal)
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
