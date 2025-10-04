package solr

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"solr-mcp-go/internal/types"
	"solr-mcp-go/internal/utils"
)

type SchemaContext struct {
	HttpClient *http.Client
	BaseURL    string
	User       string
	Pass       string
	Cache      *types.SchemaCache
}

func GetFieldCatalog(ctx context.Context, sCtx SchemaContext, collection string) (*types.FieldCatalog, error) {
	now := time.Now()
	if fc, ok := sCtx.Cache.ByCol[collection]; ok {
		if now.Sub(sCtx.Cache.LastFetch[collection]) < sCtx.Cache.TTL {
			return fc, nil
		}
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

	for _, f := range fld.Fields {
		typ := strings.ToLower(f.Type)
		switch {
		case strings.Contains(typ, "string") || strings.Contains(typ, "text"):
			fc.Texts = append(fc.Texts, f.Name)
		case strings.Contains(typ, "int") || strings.Contains(typ, "long") || strings.Contains(typ, "float") || strings.Contains(typ, "double"):
			fc.Numbers = append(fc.Numbers, f.Name)
		case strings.Contains(typ, "date"):
			fc.Dates = append(fc.Dates, f.Name)
		case strings.Contains(typ, "bool"):
			fc.Bools = append(fc.Bools, f.Name)
		}
	}
	fc.Guessed = GuessFields(fc)
	sCtx.Cache.ByCol[collection] = fc
	sCtx.Cache.LastFetch[collection] = now
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
		if err := json.NewDecoder(res.Body).Decode(into); err != nil {
			bodyBytes, _ := io.ReadAll(res.Body)
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

func GuessFields(fc *types.FieldCatalog) types.GuessedFields {
	var gf types.GuessedFields
	findBy := func(names []string, keys ...string) string {
		for _, n := range names {
			ln := strings.ToLower(n)
			for _, k := range keys {
				if strings.Contains(ln, k) {
					return n
				}
			}
		}
		return ""
	}
	gf.Price = findBy(fc.Numbers, "price", "cost", "amount", "fee", "tax", "salary", "budget", "cost", "金額", "価格", "値段", "料金")
	gf.Date = findBy(fc.Dates, "date", "time", "timestamp", "created", "updated", "modified", "日付", "日時", "時間", "登録", "更新")
	gf.Brand = findBy(fc.Texts, "brand", "maker", "manufacturer", "vendor", "supplier", "メーカー", "ブランド", "製造", "販売")
	gf.Category = findBy(fc.Texts, "category", "type", "genre", "class", "カテゴリ", "分類", "種類", "ジャンル")
	gf.InStock = findBy(fc.Bools, "in_stock", "available", "stock", "在庫", "有無", "販売")
	prior := utils.Prioritize(fc.Texts, []string{"title", "name", "product", "item", "商品名", "説明", "description", "detail", "内容", "本文", "テキスト", "text"})
	if len(prior) == 0 && len(fc.Texts) > 0 { // もし優先順位付けの結果が空なら、元のテキストフィールドリストを使う
		prior = fc.Texts
	}
	if len(prior) > 0 {
		gf.DefaultDF = prior[0]
	}
	gf.TextTopN = utils.HeadN(prior, 3)
	return gf
}

func SummarizeSchema(fc *types.FieldCatalog) string {
	sb := &strings.Builder{}
	fmt.Fprintf(sb, "uniqueKey=%s\n", fc.UniqueKey)
	write := func(tag string, xs []string) {
		if len(xs) == 0 {
			return
		}
		var describedFields []string
		for _, fieldName := range utils.HeadN(xs, 30) {
			if meta, ok := fc.Metadata[fieldName]; ok && meta.Description != "" {
				describedFields = append(describedFields, fmt.Sprintf("%s(%s)", fieldName, meta.Description))
			} else {
				describedFields = append(describedFields, fieldName)
			}
		}
		fmt.Fprintf(sb, "%s: %s\n", tag, strings.Join(describedFields, ", "))
	}
	write("text_fields", fc.Texts)
	write("number_fields", fc.Numbers)
	write("date_fields", fc.Dates)
	write("bool_fields", fc.Bools)
	if fc.Guessed.Price != "" {
		fmt.Fprintf(sb, "guess.price=%s\n", fc.Guessed.Price)
	}
	if fc.Guessed.Date != "" {
		fmt.Fprintf(sb, "guess.date=%s\n", fc.Guessed.Date)
	}
	if fc.Guessed.Brand != "" {
		fmt.Fprintf(sb, "guess.brand=%s\n", fc.Guessed.Brand)
	}
	if fc.Guessed.Category != "" {
		fmt.Fprintf(sb, "guess.category=%s\n", fc.Guessed.Category)
	}
	if fc.Guessed.InStock != "" {
		fmt.Fprintf(sb, "guess.inStock=%s\n", fc.Guessed.InStock)
	}
	if fc.Guessed.DefaultDF != "" {
		fmt.Fprintf(sb, "guess.defaultDF=%s\n", fc.Guessed.DefaultDF)
	}
	return sb.String()
}
