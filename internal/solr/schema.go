package solr

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"

	"solr-mcp-go/internal/types"
)

type SchemaContext struct {
	HttpClient *http.Client
	BaseURL    string
	User       string
	Pass       string
	Cache      *types.SchemaCache
}

func GetFieldCatalog(ctx context.Context, sCtx SchemaContext, collection string) (*types.FieldCatalog, error) {
	// Check cache with thread-safe access
	if fc, ok := sCtx.Cache.Get(collection); ok {
		return fc, nil
	}

	fc := &types.FieldCatalog{}
	ukURL := fmt.Sprintf("%s/solr/%s/schema/uniquekey?wt=json", sCtx.BaseURL, url.PathEscape(collection))
	if err := getJSON(ctx, sCtx.HttpClient, sCtx.User, sCtx.Pass, ukURL, &struct {
		UniqueKey string `json:"uniqueKey"`
	}{}, func(v any) {
		uniquekey := v.(*struct {
			UniqueKey string `json:"uniqueKey"`
		}).UniqueKey
		fc.UniqueKey = uniquekey
	}); err != nil {
		return nil, fmt.Errorf("failed to get uniqueKey from Solr: %v", err)
	}

	fieldsURL := fmt.Sprintf("%s/solr/%s/schema/fields?wt=json&includeDynamic=true", sCtx.BaseURL, url.PathEscape(collection))
	var fld struct {
		Fields []types.SolrField `json:"fields"`
	}
	if err := getJSON(ctx, sCtx.HttpClient, sCtx.User, sCtx.Pass, fieldsURL, &fld, nil); err != nil {
		return nil, fmt.Errorf("failed to get fields from Solr: %v", err)
	}
	fc.All = fld.Fields

	metadataURL := fmt.Sprintf("%s/solr/%s/admin/file?file=field_metadata.json&wt=json", sCtx.BaseURL, url.PathEscape(collection))
	var metadata map[string]types.FieldMetadata
	if err := getJSON(ctx, sCtx.HttpClient, sCtx.User, sCtx.Pass, metadataURL, &metadata, nil); err == nil {
		fc.Metadata = metadata
	} else {
		slog.Warn("failed to get field metadata from Solr", "err", err)
	}

	// Store in cache with thread-safe access
	sCtx.Cache.Set(collection, fc)
	return fc, nil
}

func getJSON(ctx context.Context, httpClient *http.Client, user, pass, u string, into any, after func(any)) error {
	slog.Info("GET", "url", u)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}
	if user != "" {
		req.SetBasicAuth(user, pass)
	}
	res, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request error: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(res.Body)
		return fmt.Errorf("HTTP status %d: %s", res.StatusCode, string(bodyBytes))
	}

	if into != nil {
		// Read body into bytes first so we can reuse it on decode error
		bodyBytes, err := io.ReadAll(res.Body)
		if err != nil {
			return fmt.Errorf("failed to read response body: %v", err)
		}
		if err := json.Unmarshal(bodyBytes, into); err != nil {
			return fmt.Errorf("JSON decode error: %v. Response: %s", err, string(bodyBytes))
		}
	} else {
		_, _ = io.Copy(io.Discard, res.Body)
	}

	if after != nil {
		after(into)
	}

	return nil
}
