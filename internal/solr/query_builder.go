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
	"strconv"
	"strings"

	"solr-mcp-go/internal/types"
	"solr-mcp-go/internal/utils"

	solr_sdk "github.com/stevenferrer/solr-go"
)

func BuildEdismaxParams(plan *types.LlmPlan, fc *types.FieldCatalog, rows, start int) map[string]any {
	p := map[string]any{
		"defType": "edismax",
		"q":       plan.EdisMax.TextQuery,
		"rows":    rows,
		"start":   start,
		"wt":      "json",
	}
	for k, v := range plan.EdisMax.Params {
		p[k] = v
	}
	if _, ok := p["df"]; !ok && fc.Guessed.DefaultDF != "" {
		p["df"] = fc.Guessed.DefaultDF
	}
	var fqs []string
	fqs = append(fqs, plan.EdisMax.Filters...)
	for _, r := range plan.EdisMax.Ranges {
		fqs = append(fqs, toRangeFilterQuery(r))
	}
	if len(fqs) > 0 {
		p["fq"] = fqs
	}
	if plan.EdisMax.Sort != "" {
		p["sort"] = plan.EdisMax.Sort
	}
	if len(plan.EdisMax.FacetFields) > 0 {
		p["facet"] = "true"
		p["facet.field"] = plan.EdisMax.FacetFields
		p["facet.limit"] = 10
		p["facet.mincount"] = 1
	}
	if len(plan.EdisMax.Fields) > 0 {
		p["fl"] = strings.Join(plan.EdisMax.Fields, ",")
	}
	return p
}

func toRangeFilterQuery(r types.LlmRange) string {
	lowerBound := "*"
	upperBound := "*"
	if r.From != nil && *r.From != "" {
		lowerBound = *r.From
	}
	if r.To != nil && *r.To != "" {
		upperBound = *r.To
	}
	return fmt.Sprintf("%s:[%s TO %s]", r.Field, lowerBound, upperBound)
}

func QuerySelect(ctx context.Context, client *solr_sdk.JSONClient, collection string, params map[string]any) (any, error) {
	q := utils.Choose(params["q"].(string), "*:*")
	query := solr_sdk.NewQuery(solr_sdk.NewStandardQueryParser().Query(q).BuildParser()).Params(solr_sdk.M(params))
	slog.Debug("Executing Solr eDisMax query on collection", "collection", collection, "query", query)
	return client.Query(ctx, collection, query)
}

func BuildKNNJSON(plan *types.LlmPlan, fc *types.FieldCatalog, embedding []float64, rows, start int) map[string]any {
	body := types.KnnJSON{
		Filter: []string{},
		KNN: []types.KnnSpec{
			{
				Field:  plan.Vector.Field,
				Vector: embedding,
				K:      utils.ChooseInt(plan.Vector.K, 100),
			},
		},
		Limit:  rows,
		Offset: start,
	}
	for _, f := range plan.EdisMax.Filters {
		body.Filter = append(body.Filter, f)
	}
	for _, r := range plan.EdisMax.Ranges {
		body.Filter = append(body.Filter, toRangeFilterQuery(r))
	}
	if strings.TrimSpace(plan.EdisMax.TextQuery) != "" && plan.EdisMax.TextQuery != "*:*" {
		qf := plan.EdisMax.Params["qf"]
		df := plan.EdisMax.Params["df"]
		locals := []string{"edismax"}
		if qf != "" {
			locals = append(locals, fmt.Sprintf("qf=%s", qf))
		}
		if df != "" && fc.Guessed.DefaultDF != "" {
			df = fc.Guessed.DefaultDF
		}
		if df != "" {
			locals = append(locals, fmt.Sprintf("df=%s", df))
		}
		body.Query = fmt.Sprintf("{!%s}%s", strings.Join(locals, " "), plan.EdisMax.TextQuery)
	}
	if plan.EdisMax.Sort != "" {
		body.Sort = plan.EdisMax.Sort
	}
	if len(plan.EdisMax.Fields) > 0 {
		body.Fields = plan.EdisMax.Fields
	}
	if len(plan.EdisMax.FacetFields) > 0 {
		ff := map[string]any{}
		for _, f := range plan.EdisMax.FacetFields {
			ff["facet_"+f] = map[string]any{
				"type":     "terms",
				"field":    f,
				"limit":    10,
				"mincount": 1,
			}
		}
		body.Facet = ff
	}
	if len(plan.EdisMax.Params) > 0 {
		body.Params = map[string]any{}
		for k, v := range plan.EdisMax.Params {
			body.Params[k] = v
		}
	}
	var m map[string]any
	b, _ := json.Marshal(body)
	_ = json.Unmarshal(b, &m)
	return m
}

func PostQueryJSON(ctx context.Context, httpClient *http.Client, baseURL, user, pass, collection string, body map[string]any) (map[string]any, error) {
	u := fmt.Sprintf("%s/solr/%s/query?wt=json", baseURL, url.PathEscape(collection))
	buf, _ := json.Marshal(body)
	slog.Debug("POST with body", "url", u, "body", string(buf))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(buf))
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
