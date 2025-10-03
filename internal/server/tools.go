package server

import (
    "context"
    "errors"
    "fmt"
    "log/slog"
    "strings"

    "solr-mcp-go/internal/llm"
    "solr-mcp-go/internal/solr"
    "solr-mcp-go/internal/types"
    "solr-mcp-go/internal/utils"

    "github.com/modelcontextprotocol/go-sdk/mcp"
    solr_sdk "github.com/stevenferrer/solr-go"
)

func AddTools(mcpServer *mcp.Server, st *State) {
    mcp.AddTool(mcpServer, &mcp.Tool{
        Name:        "solr.query",
        Description: "Search documents in Solr /select query",
    }, st.toolQuery)

    mcp.AddTool(mcpServer, &mcp.Tool{
        Name:        "solr.commit",
        Description: "Hard Commit changes to Solr",
    }, st.toolCommit)

    mcp.AddTool(mcpServer, &mcp.Tool{
        Name:        "solr.ping",
        Description: "Health Check",
    }, st.toolPing)

    mcp.AddTool(mcpServer, &mcp.Tool{
        Name:        "solr.searchSmart",
        Description: "Convert natural language query to Solr query using LLM",
    }, st.toolSearchSmart)
}

// Basic Tools
func (st *State) toolQuery(ctx context.Context, _ *mcp.CallToolRequest, in types.QueryIn) (*mcp.CallToolResult, any, error) {
    col, err := st.ColOrDefault(in.Collection)
    if err != nil {
        return nil, nil, err
    }
    qString := in.Query
    if qString == "" {
        qString = "*:*"
    }

    parser := solr_sdk.NewStandardQueryParser().Query(qString).BuildParser()
    query := solr_sdk.NewQuery(parser)
    if len(in.Fields) > 0 {
        query = query.Fields(in.Fields...)
    }
    if len(in.FilterQuery) > 0 {
        query = query.Filters(in.FilterQuery...)
    }
    if in.Sort != "" {
        query = query.Sort(in.Sort)
    }
    if in.Start != nil {
        query = query.Offset(*in.Start)
    }
    if in.Rows != nil {
        query = query.Limit(*in.Rows)
    }
    if len(in.Params) > 0 {
        query = query.Params(solr_sdk.M(in.Params))
    }

    slog.Debug("Executing Solr query", "collection", col, "query", query)
    resp, err := st.SolrClient.Query(ctx, col, query)
    return nil, resp, err
}

func (st *State) toolCommit(ctx context.Context, _ *mcp.CallToolRequest, in types.CommitIn) (*mcp.CallToolResult, any, error) {
    col, err := st.ColOrDefault(in.Collection)
    if err != nil {
        return nil, nil, err
    }
    slog.Info("Committing to Solr", "collection", col)
    err = st.SolrClient.Commit(ctx, col)
    return nil, map[string]any{"ok": err == nil}, err
}

func (st *State) toolPing(ctx context.Context, _ *mcp.CallToolRequest, in types.PingIn) (*mcp.CallToolResult, any, error) {
    col, err := st.ColOrDefault(in.Collection)
    if err != nil {
        return nil, nil, err
    }
    query := solr_sdk.NewQuery(solr_sdk.NewStandardQueryParser().Query("*:*").BuildParser()).Limit(0)
    resp, err := st.SolrClient.Query(ctx, col, query)
    if err != nil {
        slog.Error("Ping error", "response", resp, "error", err)
        return nil, nil, err
    }
    return nil, map[string]any{
        "status": resp.BaseResponse.Header.Status,
        "qtime":  resp.BaseResponse.Header.QTime,
    }, nil
}

// Smart Search Tool
func (st *State) toolSearchSmart(ctx context.Context, _ *mcp.CallToolRequest, in types.SearchSmartIn) (*mcp.CallToolResult, any, error) {
    if strings.TrimSpace(in.Query) == "" {
        return nil, nil, errors.New("input.query is required")
    }
    col, err := st.ColOrDefault(in.Collection)
    if err != nil {
        return nil, nil, err
    }
    locale := in.Locale
    if locale == "" {
        locale = "ja"
    }
    allowVector := true
    if in.AllowVector != nil {
        allowVector = *in.AllowVector
    }
    allowHybrid := true
    if in.AllowHybrid != nil {
        allowHybrid = *in.AllowHybrid
    }

    sCtx := solr.SchemaContext{
        HttpClient: st.HttpClient,
        BaseURL:    st.BaseURL,
        User:       st.BasicUser,
        Pass:       st.BasicPass,
        Cache:      &st.SchemaCache,
    }
    fc, err := solr.GetFieldCatalog(ctx, sCtx, col)
    if err != nil {
        return nil, nil, fmt.Errorf("failed to get schema: %v", err)
    }
    summary := solr.SummarizeSchema(fc)
    slog.Debug("Generated schema summary for LLM", "summary", summary)

    if st.LlmBaseURL == "" || st.LlmAPIKey == "" {
        return nil, nil, errors.New("LLM_BASE_URL and LLM_API_KEY must be set for smart search")
    }

    llmCfg := llm.LLMConfig{
        HttpClient: st.HttpClient,
        BaseURL:    st.LlmBaseURL,
        APIKey:     st.LlmAPIKey,
        Model:      st.LlmModel,
    }
    plan, planJSON, err := llm.CallLLMForPlan(ctx, llmCfg, in.Query, locale, summary, allowVector, allowHybrid)
    if err != nil {
        return nil, nil, fmt.Errorf("failed to get plan from LLM: %v", err)
    }

    rows := 20
    if in.Rows != nil {
        rows = *in.Rows
    }
    start := 0
    if in.Start != nil {
        start = *in.Start
    }

    switch strings.ToLower(plan.Mode) {
    case "vector":
        if !allowVector {
            return nil, nil, errors.New("plan requires vector search but input.allowVector is false")
        }
        embCfg := llm.EmbeddingConfig{
            HttpClient: st.HttpClient,
            BaseURL:    st.EmbeddingBaseURL,
            APIKey:     st.EmbeddingAPIKey,
            Model:      st.EmbeddingModel,
        }
        embedding, err := llm.EnsureEmbedding(ctx, embCfg, utils.Choose(plan.Vector.QueryText, in.Query))
        if err != nil {
            return nil, nil, fmt.Errorf("failed to get embedding: %v", err)
        }
        jsonReq := solr.BuildKNNJSON(plan, fc, embedding, rows, start)
        resp, err := solr.PostQueryJSON(ctx, st.HttpClient, st.BaseURL, st.BasicUser, st.BasicPass, col, jsonReq)
        if err != nil {
            return nil, nil, fmt.Errorf("failed to execute vector query: %v", err)
        }
        return nil, types.SearchSmartOut{
            Plan:           planJSON,
            JSONRequest:    jsonReq,
            Response:       resp,
            SchemaGuesses:  fc.Guessed,
            ExecutionNotes: "Executed JSON Request API with KNN (vector search).",
        }, nil
    case "hybrid":
        if !(allowHybrid && allowVector) {
            return nil, nil, errors.New("plan requires hybrid search but input.allowHybrid or input.allowVector is false")
        }
        embCfg := llm.EmbeddingConfig{
            HttpClient: st.HttpClient,
            BaseURL:    st.EmbeddingBaseURL,
            APIKey:     st.EmbeddingAPIKey,
            Model:      st.EmbeddingModel,
        }
        embedding, err := llm.EnsureEmbedding(ctx, embCfg, utils.Choose(plan.Vector.QueryText, in.Query))
        if err != nil {
            return nil, nil, fmt.Errorf("failed to get embedding: %v", err)
        }
        knnOnly := solr.BuildKNNJSON(plan, fc, embedding, utils.ChooseInt(plan.Vector.K, 200), 0)
        solr.AddFieldsForIDs(knnOnly, fc.UniqueKey)
        knnResp, err := solr.PostQueryJSON(ctx, st.HttpClient, st.BaseURL, st.BasicUser, st.BasicPass, col, knnOnly)
        if err != nil {
            return nil, nil, fmt.Errorf("failed to execute vector part of hybrid query: %v", err)
        }
        ids := solr.ExtractIDs(knnResp, fc.UniqueKey)
        if len(ids) == 0 {
            selectParams := solr.BuildEdismaxParams(plan, fc, rows, start)
            resp, err := solr.QuerySelect(ctx, st.SolrClient, col, selectParams)
            if err != nil {
                return nil, nil, fmt.Errorf("failed to execute keyword part of hybrid query: %v", err)
            }
            return nil, types.SearchSmartOut{
                Plan:           planJSON,
                SelectParams:   selectParams,
                Response:       resp,
                SchemaGuesses:  fc.Guessed,
                ExecutionNotes: "Hybrid search: Vector part returned 0 results, so returned only keyword search results.",
            }, nil
        }
        selectParams := solr.BuildEdismaxParams(plan, fc, rows, start)
        idFilterQuery := fmt.Sprintf("%s:(%s)", fc.UniqueKey, strings.Join(ids, " OR "))
        solr.AppendFilterQuery(selectParams, idFilterQuery)
        resp, err := solr.QuerySelect(ctx, st.SolrClient, col, selectParams)
        if err != nil {
            return nil, nil, fmt.Errorf("failed to execute keyword part of hybrid query: %v", err)
        }
        return nil, types.SearchSmartOut{
            Plan:           planJSON,
            SelectParams:   selectParams,
            Response:       resp,
            SchemaGuesses:  fc.Guessed,
            ExecutionNotes: fmt.Sprintf("Hybrid search: Vector part returned %d results, combined with keyword search.", len(ids)),
        }, nil
    default: // "keyword"
        selectParams := solr.BuildEdismaxParams(plan, fc, rows, start)
        resp, err := solr.QuerySelect(ctx, st.SolrClient, col, selectParams)
        if err != nil {
            return nil, nil, fmt.Errorf("failed to execute keyword query: %v", err)
        }
        return nil, types.SearchSmartOut{
            Plan:           planJSON,
            SelectParams:   selectParams,
            Response:       resp,
            SchemaGuesses:  fc.Guessed,
            ExecutionNotes: "Executed eDisMax keyword search.",
        }, nil
    }
}