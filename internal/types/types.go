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
	EchoParams  bool           `json:"echoParams,omitempty"`
}

type CommitIn struct {
	Collection string `json:"collection,omitempty"`
}

type PingIn struct {
	// No fields needed - cluster-wide ping
}

type CollectionHealthIn struct {
	Collection string `json:"collection,omitempty"`
}

// Smart search tool types
type SchemaIn struct {
	Collection string `json:"collection,omitempty"`
}

type SchemaOut struct {
	SelectParams   map[string]any `json:"selectParams,omitempty"`   // 実際に実行した/selectのパラメータ
	JSONRequest    any            `json:"jsonRequest,omitempty"`    // 実際に実行したJSONリクエストボディ
	Response       any            `json:"response,omitempty"`       // Solrからのレスポンス
	ExecutionNotes string         `json:"executionNotes,omitempty"` // 実行経路の説明
}
