package solr

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"solr-mcp-go/internal/utils"
	"strconv"

	solr_sdk "github.com/stevenferrer/solr-go"
)

func QuerySelect(ctx context.Context, client *solr_sdk.JSONClient, collection string, params map[string]any) (any, error) {
	q := utils.Choose(params["q"].(string), "*:*")
	query := solr_sdk.NewQuery(solr_sdk.NewStandardQueryParser().Query(q).BuildParser()).Params(solr_sdk.M(params))
	slog.Debug("Executing Solr eDisMax query on collection", "collection", collection, "query", query)
	return client.Query(ctx, collection, query)
}

func PostQueryJSON(ctx context.Context, httpClient *http.Client, baseURL, user, pass, collection string, body map[string]any) (map[string]any, error) {
	u := fmt.Sprintf("%s/solr/%s/query?wt=json", baseURL, url.PathEscape(collection))
	buf, _ := json.Marshal(body)
	slog.Debug("POST with body", "url", u, "body", string(buf))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(buf))
	if err != nil {
		return nil, fmt.Errorf("create request error: %v", err)
	}
	if user != "" {
		req.SetBasicAuth(user, pass)
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request error: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("HTTP status %d: %s", res.StatusCode, string(bodyBytes))
	}

	var out map[string]any
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("JSON decode error: %v", err)
	}
	return out, nil
}

func AddFieldsForIDs(body map[string]any, idField string) {
	if idField == "" {
		return
	}
	if _, ok := body["fields"]; !ok {
		return
	}
	body["fields"] = []string{idField}
}

func ExtractIDs(resp map[string]any, idField string) []string {
	var ids []string
	respObj, _ := resp["response"].(map[string]any)
	if respObj == nil {
		return ids
	}
	docs, _ := respObj["docs"].([]any)
	for _, d := range docs {
		m, _ := d.(map[string]any)
		if v, ok := m[idField]; ok {
			switch t := v.(type) {
			case string:
				ids = append(ids, t)
			case float64:
				ids = append(ids, strconv.FormatInt(int64(t), 10))
			}
		}
	}
	return ids
}

func AppendFilterQuery(params map[string]any, fq string) {
	switch cur := params["fq"].(type) {
	case nil:
		params["fq"] = []string{fq}
	case string:
		params["fq"] = []string{cur, fq}
	case []string:
		params["fq"] = append(cur, fq)
	case []any:
		params["fq"] = append(cur, fq)
	}
}

// QueryWithRawResponse executes a query and returns the raw JSON response as map[string]any
// This preserves all fields from Solr response including params in responseHeader
func QueryWithRawResponse(ctx context.Context, httpClient *http.Client, baseURL, user, pass, collection string, query *solr_sdk.Query) (map[string]any, error) {
	// Build the query URL
	queryURL := fmt.Sprintf("%s/solr/%s/select", baseURL, url.PathEscape(collection))

	// Convert query to URL parameters
	queryMap := query.BuildQuery()
	values := url.Values{}
	for k, v := range queryMap {
		// Convert JSON query API format to traditional /select format
		// "query" -> "q", "limit" -> "rows", "offset" -> "start"
		paramKey := k
		switch k {
		case "query":
			paramKey = "q"
		case "limit":
			paramKey = "rows"
		case "offset":
			paramKey = "start"
		case "fields":
			paramKey = "fl"
		case "filter":
			paramKey = "fq"
		}

		switch val := v.(type) {
		case string:
			values.Add(paramKey, val)
		case []string:
			for _, s := range val {
				values.Add(paramKey, s)
			}
		case []any:
			for _, s := range val {
				values.Add(paramKey, fmt.Sprintf("%v", s))
			}
		case int:
			values.Add(paramKey, strconv.Itoa(val))
		case int64:
			values.Add(paramKey, strconv.FormatInt(val, 10))
		case float64:
			values.Add(paramKey, strconv.FormatFloat(val, 'f', -1, 64))
		case bool:
			values.Add(paramKey, strconv.FormatBool(val))
		case map[string]any:
			// Handle nested map (like params) by flattening it
			for subKey, subVal := range val {
				switch subV := subVal.(type) {
				case string:
					values.Add(subKey, subV)
				case []string:
					for _, s := range subV {
						values.Add(subKey, s)
					}
				default:
					values.Add(subKey, fmt.Sprintf("%v", subV))
				}
			}
		default:
			slog.Warn("Unexpected query parameter type", "key", k, "type", fmt.Sprintf("%T", val), "value", val)
			values.Add(paramKey, fmt.Sprintf("%v", val))
		}
	}
	values.Set("wt", "json")

	fullURL := queryURL + "?" + values.Encode()
	slog.Debug("Executing raw Solr query", "url", fullURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %v", err)
	}

	if user != "" {
		req.SetBasicAuth(user, pass)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("JSON decode error: %v", err)
	}

	return result, nil
}
