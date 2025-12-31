package types

import (
	"sync"
	"time"
)

// Schema related types
type SchemaCache struct {
	mu        sync.RWMutex
	LastFetch map[string]time.Time
	TTL       time.Duration
	ByCol     map[string]*FieldCatalog
}

// Get retrieves a cached FieldCatalog if it exists and is still valid
func (sc *SchemaCache) Get(collection string) (*FieldCatalog, bool) {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	fc, ok := sc.ByCol[collection]
	if !ok {
		return nil, false
	}

	lastFetch, ok := sc.LastFetch[collection]
	if !ok {
		return nil, false
	}

	if time.Since(lastFetch) >= sc.TTL {
		return nil, false
	}

	return fc, true
}

// Set stores a FieldCatalog in the cache
func (sc *SchemaCache) Set(collection string, fc *FieldCatalog) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	sc.ByCol[collection] = fc
	sc.LastFetch[collection] = time.Now()
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
	SelectParams   map[string]any `json:"selectParams,omitempty"`   // Parameters used for the executed /select request
	JSONRequest    any            `json:"jsonRequest,omitempty"`    // Executed JSON request body
	Response       any            `json:"response,omitempty"`       // Response returned from Solr
	ExecutionNotes string         `json:"executionNotes,omitempty"` // Explanation of the execution path
}
