package types

import (
    "time"
)

// Schema related types
type SchemaCache struct {
    LastFetch map[string]time.Time
    TTL       time.Duration
    ByCol     map[string]*FieldCatalog
}

type FieldCatalog struct {
    UniqueKey string
    All       []SolrField
    Texts     []string
    Numbers   []string
    Dates     []string
    Bools     []string
    Guessed   GuessedFields
    Metadata  map[string]FieldMetadata `json:"metadata,omitempty"`
}

type SolrField struct {
    Name        string `json:"name"`
    Type        string `json:"type"`
    Indexed     bool   `json:"indexed"`
    Stored      bool   `json:"stored"`
    MultiValued bool   `json:"multiValued,omitempty"`
}

type FieldMetadata struct {
    Description string `json:"description"`
}

type GuessedFields struct {
    Price     string
    Date      string
    Brand     string
    Category  string
    InStock   string
    DefaultDF string
    TextTopN  []string
}

// Basic tool types
type QueryIn struct {
    Collection  string         `json:"collection,omitempty"`
    Query       string         `json:"query,omitempty"`
    FilterQuery []string       `json:"fq,omitempty"`
    Fields      []string       `json:"fl,omitempty"`
    Sort        string         `json:"sort,omitempty"`
    Start       *int           `json:"start,omitempty"`
    Rows        *int           `json:"rows,omitempty"`
    Params      map[string]any `json:"params,omitempty"`
}

type CommitIn struct {
    Collection string `json:"collection,omitempty"`
}

type PingIn struct {
    Collection string `json:"collection,omitempty"`
}

// Smart search tool types
type SearchSmartIn struct {
    Collection  string `json:"collection,omitempty"`
    Query       string `json:"query,omitempty"`
    Locale      string `json:"locale,omitempty"` // ja, en, ...
    Rows        *int   `json:"rows,omitempty"`
    Start       *int   `json:"start,omitempty"`
    AllowVector *bool  `json:"allowVector,omitempty"` // default: true
    AllowHybrid *bool  `json:"allowHybrid,omitempty"` // default: true
}

type SearchSmartOut struct {
    Plan           any           `json:"plan"`                   // LLMが生成したSolrクエリプラン（JSON形式）
    SelectParams   map[string]any `json:"selectParams,omitempty"` // 実際に実行した/selectのパラメータ
    JSONRequest    any           `json:"jsonRequest,omitempty"`  // 実際に実行したJSONリクエストボディ
    Response       any           `json:"response,omitempty"`     // Solrからのレスポンス
    SchemaGuesses  GuessedFields `json:"schemaGuesses,omitempty"`  // スキーマ推定結果
    ExecutionNotes string        `json:"executionNotes,omitempty"` // 実行経路の説明
}

// LLM related types
type LlmPlan struct {
    Mode string `json:"mode"` // "keyword", "vector", "hybrid"

    EdisMax struct {
        TextQuery   string         `json:"text_query,omitempty"`
        Filters     []string       `json:"filters,omitempty"`
        Ranges      []LlmRange     `json:"ranges,omitempty"`
        Sort        string         `json:"sort,omitempty"`
        FacetFields []string       `json:"facet_fields,omitempty"`
        Params      map[string]any `json:"params,omitempty"` // qf, pf, mm, tie, df, ...
        Fields      []string       `json:"fields,omitempty"`
    } `json:"edismax"`

    Vector struct {
        Field     string `json:"field,omitempty"`
        K         int    `json:"k,omitempty"`
        QueryText string `json:"query_text,omitempty"`
    } `json:"vector"`

    Rows  *int `json:"rows,omitempty"`
    Start *int `json:"start,omitempty"`
}

type LlmRange struct {
    Field        string  `json:"field"`
    Type         string  `json:"type"` // "date", "number"
    From         *string `json:"from,omitempty"` // inclusive lower bound; "*" allowed
    To           *string `json:"to,omitempty"`   // inclusive upper bound; "*" allowed
    IncludeLower *bool   `json:"includeLower,omitempty"` // default true
    IncludeUpper *bool   `json:"includeUpper,omitempty"` // default true
}

// Solr JSON query types
type KnnJSON struct {
    Query  string         `json:"query"`
    Filter []string       `json:"filter,omitempty"`
    KNN    []KnnSpec      `json:"knn,omitempty"`
    Fields []string       `json:"fields,omitempty"`
    Limit  int            `json:"limit,omitempty"`
    Offset int            `json:"offset,omitempty"`
    Sort   string         `json:"sort,omitempty"`
    Params map[string]any `json:"params,omitempty"`
    Facet  any            `json:"facet,omitempty"`
}

type KnnSpec struct {
    Field  string    `json:"field"`
    Vector []float64 `json:"vector"`
    K      int       `json:"k"`
}