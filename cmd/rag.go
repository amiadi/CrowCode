package cmd

import (
	"context"
	stdsql "database/sql"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/db"
	ragext "github.com/opencode-ai/opencode/internal/rag"
	"github.com/spf13/cobra"

	_ "github.com/ncruces/go-sqlite3/driver"
)

var ragCmd = &cobra.Command{
	Use:   "rag",
	Short: "RAG indexing and retrieval commands",
}

var ragIndexCmd = &cobra.Command{
	Use:   "index",
	Short: "Index text into SQLite-backed RAG store",
	RunE: func(cmd *cobra.Command, args []string) error {
		svc, cleanup, err := ragServiceFromCommand(cmd)
		if err != nil {
			return err
		}
		defer cleanup()

		sourceType, _ := cmd.Flags().GetString("source-type")
		uri, _ := cmd.Flags().GetString("uri")
		name, _ := cmd.Flags().GetString("name")
		externalID, _ := cmd.Flags().GetString("external-id")
		title, _ := cmd.Flags().GetString("title")
		text, _ := cmd.Flags().GetString("text")
		textFile, _ := cmd.Flags().GetString("text-file")
		sqlDSN, _ := cmd.Flags().GetString("sql-dsn")
		sqlQuery, _ := cmd.Flags().GetString("sql-query")
		objectURI, _ := cmd.Flags().GetString("object-uri")
		directoryPath, _ := cmd.Flags().GetString("directory-path")
		recursive, _ := cmd.Flags().GetBool("recursive")
		maxFiles, _ := cmd.Flags().GetInt64("max-files")
		maxBytes, _ := cmd.Flags().GetInt64("max-bytes")
		chunkSize, _ := cmd.Flags().GetInt("chunk-size")
		chunkOverlap, _ := cmd.Flags().GetInt("chunk-overlap")
		embeddingModel, _ := cmd.Flags().GetString("embedding-model")

		if strings.TrimSpace(uri) == "" {
			return fmt.Errorf("--uri is required")
		}

		resolvedText, resolvedTitle, resolvedExternalID, err := resolveIndexPayload(
			context.Background(),
			sourceType,
			uri,
			text,
			textFile,
			sqlDSN,
			sqlQuery,
			objectURI,
			directoryPath,
			recursive,
			maxFiles,
			maxBytes,
		)
		if err != nil {
			return err
		}
		if strings.TrimSpace(resolvedText) == "" {
			return fmt.Errorf("provide content via --text/--text-file or connector flags for source-type %s", sourceType)
		}
		if title == "" {
			title = resolvedTitle
		}
		if externalID == "" {
			externalID = resolvedExternalID
		}

		result, err := svc.IndexText(context.Background(), ragext.IndexParams{
			SourceType:     sourceType,
			URI:            uri,
			Name:           name,
			ExternalID:     externalID,
			Title:          title,
			Text:           resolvedText,
			ChunkSize:      chunkSize,
			ChunkOverlap:   chunkOverlap,
			EmbeddingModel: embeddingModel,
		})
		if err != nil {
			return err
		}

		fmt.Printf("Indexed source=%s document=%s chunks=%d\n", result.SourceID, result.DocumentID, result.ChunksCreated)
		return nil
	},
}

func resolveIndexPayload(
	ctx context.Context,
	sourceType, uri, text, textFile, sqlDSN, sqlQuery, objectURI, directoryPath string,
	recursive bool,
	maxFiles, maxBytes int64,
) (string, string, string, error) {
	if strings.TrimSpace(text) != "" {
		return text, uri, uri, nil
	}

	if strings.TrimSpace(textFile) != "" {
		content, readErr := ragext.ExtractTextFromFile(textFile)
		if readErr != nil {
			return "", "", "", fmt.Errorf("failed extracting text-file content: %w", readErr)
		}
		return content, filepath.Base(textFile), textFile, nil
	}

	source := strings.ToLower(strings.TrimSpace(sourceType))
	switch source {
	case "web":
		body, err := fetchWebContent(ctx, uri)
		if err != nil {
			return "", "", "", err
		}
		return body, uri, uri, nil
	case "sql":
		rowsText, err := fetchSQLContent(ctx, sqlDSN, sqlQuery)
		if err != nil {
			return "", "", "", err
		}
		extID := uri
		if extID == "" {
			extID = sqlQuery
		}
		return rowsText, "sql-query", extID, nil
	case "object_storage":
		target := strings.TrimSpace(objectURI)
		if target == "" {
			target = uri
		}
		content, err := fetchObjectContent(ctx, target)
		if err != nil {
			return "", "", "", err
		}
		return content, target, target, nil
	case "directory", "folder", "local_folder", "remote_share":
		target := strings.TrimSpace(directoryPath)
		if target == "" {
			target = uri
		}
		content, resolvedPath, err := fetchDirectoryContent(ctx, target, recursive, maxFiles, maxBytes)
		if err != nil {
			return "", "", "", err
		}
		return content, filepath.Base(resolvedPath), resolvedPath, nil
	default:
		return "", "", "", nil
	}
}

func normalizeSharePath(raw string) string {
	target := normalizeLocalFilePathFromURI(strings.TrimSpace(raw))
	lower := strings.ToLower(target)

	if strings.HasPrefix(lower, "smb://") {
		target = "//" + strings.TrimPrefix(target, "smb://")
	}

	if strings.HasPrefix(target, `\\`) {
		target = "//" + strings.TrimPrefix(target, `\\`)
	}

	if strings.Contains(target, `\`) {
		target = strings.ReplaceAll(target, `\`, "/")
	}

	return filepath.Clean(target)
}

func normalizeLocalFilePathFromURI(raw string) string {
	target := strings.TrimSpace(raw)
	if !strings.HasPrefix(strings.ToLower(target), "file://") {
		return target
	}

	parsed, err := url.Parse(target)
	if err != nil {
		return strings.TrimPrefix(target, "file://")
	}

	// file://server/share/path -> //server/share/path (UNC)
	if parsed.Host != "" {
		if len(parsed.Host) == 2 && parsed.Host[1] == ':' {
			// Handle the permissive but common form file://C:/path.
			return parsed.Host + parsed.Path
		}
		return "//" + parsed.Host + parsed.Path
	}

	path := parsed.Path
	decoded, decodeErr := url.PathUnescape(path)
	if decodeErr == nil {
		path = decoded
	}

	// file:///C:/path -> C:/path
	if len(path) >= 3 && path[0] == '/' && path[2] == ':' {
		path = path[1:]
	}

	return path
}

func fetchDirectoryContent(ctx context.Context, directoryPath string, recursive bool, maxFiles, maxBytes int64) (string, string, error) {
	target := normalizeSharePath(directoryPath)
	if strings.TrimSpace(target) == "" {
		return "", "", fmt.Errorf("directory connector requires --directory-path or --uri")
	}

	if maxFiles <= 0 {
		maxFiles = 500
	}
	if maxBytes <= 0 {
		maxBytes = 16 * 1024 * 1024
	}

	info, err := os.Stat(target)
	if err != nil {
		return "", "", fmt.Errorf("failed to access directory path %s: %w", target, err)
	}
	if !info.IsDir() {
		return "", "", fmt.Errorf("directory connector path is not a folder: %s", target)
	}

	builder := strings.Builder{}
	var filesRead int64
	var bytesRead int64

	visit := func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if path == target {
			return nil
		}

		if d.IsDir() {
			if !recursive {
				return filepath.SkipDir
			}
			return nil
		}

		if !d.Type().IsRegular() {
			return nil
		}

		if filesRead >= maxFiles || bytesRead >= maxBytes {
			return io.EOF
		}

		extracted, err := ragext.ExtractTextFromFile(path)
		if err != nil {
			return nil
		}
		extracted = strings.TrimSpace(extracted)
		if extracted == "" {
			return nil
		}

		rel, err := filepath.Rel(target, path)
		if err != nil {
			rel = path
		}
		if filesRead > 0 {
			builder.WriteString("\n\n")
		}
		builder.WriteString("# file: ")
		builder.WriteString(filepath.ToSlash(rel))
		builder.WriteString("\n")
		builder.WriteString(extracted)

		filesRead++
		bytesRead += int64(len(extracted))
		return nil
	}

	err = filepath.WalkDir(target, visit)
	if err != nil && err != io.EOF {
		return "", "", err
	}

	if filesRead == 0 {
		return "", "", fmt.Errorf("no readable files found under %s (supported formats: %s)", target, ragext.SupportedExtractionSummary())
	}

	return builder.String(), target, nil
}

func fetchWebContent(ctx context.Context, pageURL string) (string, error) {
	if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(pageURL)), "http://") && !strings.HasPrefix(strings.ToLower(strings.TrimSpace(pageURL)), "https://") {
		return "", fmt.Errorf("web source requires http/https uri")
	}
	client := &http.Client{Timeout: 20 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("web fetch failed with status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8*1024*1024))
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func fetchSQLContent(ctx context.Context, dsn, query string) (string, error) {
	if strings.TrimSpace(dsn) == "" {
		return "", fmt.Errorf("sql source requires --sql-dsn")
	}
	if strings.TrimSpace(query) == "" {
		return "", fmt.Errorf("sql source requires --sql-query")
	}
	dbConn, err := stdsql.Open("sqlite3", dsn)
	if err != nil {
		return "", err
	}
	defer dbConn.Close()

	rows, err := dbConn.QueryContext(ctx, query)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return "", err
	}

	records := make([]map[string]any, 0)
	for rows.Next() {
		values := make([]any, len(cols))
		valuePtrs := make([]any, len(cols))
		for i := range values {
			valuePtrs[i] = &values[i]
		}
		if err := rows.Scan(valuePtrs...); err != nil {
			return "", err
		}

		rowMap := make(map[string]any, len(cols))
		for i, col := range cols {
			v := values[i]
			if b, ok := v.([]byte); ok {
				rowMap[col] = string(b)
			} else {
				rowMap[col] = v
			}
		}
		records = append(records, rowMap)
	}
	if err := rows.Err(); err != nil {
		return "", err
	}

	blob, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return "", err
	}
	return string(blob), nil
}

func fetchObjectContent(ctx context.Context, objectURI string) (string, error) {
	target := strings.TrimSpace(objectURI)
	if target == "" {
		return "", fmt.Errorf("object_storage source requires --object-uri or --uri")
	}

	lower := strings.ToLower(target)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		return fetchWebContent(ctx, target)
	}
	if strings.HasPrefix(lower, "s3://") {
		return "", fmt.Errorf("s3:// object URIs require a presigned https url in --object-uri for this build")
	}

	target = normalizeLocalFilePathFromURI(target)

	content, err := ragext.ExtractTextFromFile(target)
	if err != nil {
		return "", err
	}
	return content, nil
}

var ragSearchCmd = &cobra.Command{
	Use:   "search",
	Short: "Run hybrid lexical+vector search against indexed chunks",
	RunE: func(cmd *cobra.Command, args []string) error {
		svc, cleanup, err := ragServiceFromCommand(cmd)
		if err != nil {
			return err
		}
		defer cleanup()

		query, _ := cmd.Flags().GetString("query")
		limit, _ := cmd.Flags().GetInt64("limit")
		if strings.TrimSpace(query) == "" {
			return fmt.Errorf("--query is required")
		}

		results, err := svc.HybridSearch(context.Background(), query, limit)
		if err != nil {
			return err
		}
		for idx, result := range results {
			content := strings.ReplaceAll(result.Chunk.Content, "\n", " ")
			if len(content) > 140 {
				content = content[:140] + "..."
			}
			fmt.Printf("%d. hybrid=%.4f lexical=%.4f vector=%.4f source=%s doc=%s chunk=%d\n   %s\n",
				idx+1,
				result.HybridScore,
				result.LexicalScore,
				result.VectorScore,
				result.Chunk.SourceID,
				result.Chunk.ExternalID,
				result.Chunk.ChunkIndex,
				content,
			)
		}
		return nil
	},
}

var ragBenchmarkCmd = &cobra.Command{
	Use:   "benchmark",
	Short: "Benchmark hybrid retrieval latency and proxy quality",
	RunE: func(cmd *cobra.Command, args []string) error {
		svc, cleanup, err := ragServiceFromCommand(cmd)
		if err != nil {
			return err
		}
		defer cleanup()

		query, _ := cmd.Flags().GetString("query")
		limit, _ := cmd.Flags().GetInt64("limit")
		runs, _ := cmd.Flags().GetInt("runs")
		expectedTerm, _ := cmd.Flags().GetString("expected-term")
		if strings.TrimSpace(query) == "" {
			return fmt.Errorf("--query is required")
		}

		report, err := svc.BenchmarkSearch(context.Background(), query, limit, runs, expectedTerm)
		if err != nil {
			return err
		}

		fmt.Printf("runs=%d limit=%d avg_ms=%.2f p50_ms=%.2f p95_ms=%.2f proxy_quality=%.4f\n",
			report.Runs,
			report.Limit,
			report.AvgLatencyMs,
			report.P50LatencyMs,
			report.P95LatencyMs,
			report.AvgProxyQualityScore,
		)
		return nil
	},
}

func ragServiceFromCommand(cmd *cobra.Command) (*ragext.Service, func(), error) {
	debug, _ := cmd.Flags().GetBool("debug")
	cwd, _ := cmd.Flags().GetString("cwd")
	if cwd != "" {
		if err := os.Chdir(cwd); err != nil {
			return nil, nil, fmt.Errorf("failed to change directory: %w", err)
		}
	}
	if cwd == "" {
		currentWD, err := os.Getwd()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get current working directory: %w", err)
		}
		cwd = currentWD
	}

	loadedCfg, err := config.Load(cwd, debug)
	if err != nil {
		return nil, nil, err
	}

	conn, err := db.Connect()
	if err != nil {
		return nil, nil, err
	}

	queries, err := db.Prepare(context.Background(), conn)
	if err != nil {
		_ = conn.Close()
		return nil, nil, err
	}

	cleanup := func() {
		_ = queries.Close()
		_ = conn.Close()
	}

	return ragext.NewServiceFromConfig(queries, loadedCfg.RAG), cleanup, nil
}

func init() {
	ragCmd.AddCommand(ragIndexCmd)
	ragCmd.AddCommand(ragSearchCmd)
	ragCmd.AddCommand(ragBenchmarkCmd)
	rootCmd.AddCommand(ragCmd)

	for _, subcmd := range []*cobra.Command{ragIndexCmd, ragSearchCmd, ragBenchmarkCmd} {
		subcmd.Flags().Bool("debug", false, "Debug")
		subcmd.Flags().String("cwd", "", "Current working directory")
	}

	ragIndexCmd.Flags().String("source-type", "manual", "Source type (local_file, web, sql, object_storage, directory, remote_share, manual)")
	ragIndexCmd.Flags().String("uri", "", "Unique source URI")
	ragIndexCmd.Flags().String("name", "", "Human-friendly source name")
	ragIndexCmd.Flags().String("external-id", "", "External document identifier")
	ragIndexCmd.Flags().String("title", "", "Document title")
	ragIndexCmd.Flags().String("text", "", "Raw text to index")
	ragIndexCmd.Flags().String("text-file", "", "Path to a file to extract and index (txt, md, json, yaml, xml, html, csv, tsv, docx, xlsx, pdf)")
	ragIndexCmd.Flags().String("sql-dsn", "", "SQL connector DSN (sqlite3 for now)")
	ragIndexCmd.Flags().String("sql-query", "", "SQL query to extract text rows for indexing")
	ragIndexCmd.Flags().String("object-uri", "", "Object storage URI (file:// local file or presigned https URL)")
	ragIndexCmd.Flags().String("directory-path", "", "Directory or share path (Linux mount, Windows UNC, file://, or smb://)")
	ragIndexCmd.Flags().Bool("recursive", true, "Recursively read nested folders for directory/share connectors")
	ragIndexCmd.Flags().Int64("max-files", 500, "Maximum files to ingest from directory/share connectors")
	ragIndexCmd.Flags().Int64("max-bytes", 16777216, "Maximum total bytes to ingest from directory/share connectors")
	ragIndexCmd.Flags().Int("chunk-size", 1200, "Chunk size in characters")
	ragIndexCmd.Flags().Int("chunk-overlap", 200, "Chunk overlap in characters")
	ragIndexCmd.Flags().String("embedding-model", "crowcode-hash-128", "Embedding model label")

	backfillCmd := &cobra.Command{
		Use:   "backfill-embeddings",
		Short: "Backfill previously indexed hash embeddings to active embedding backend",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, cleanup, err := ragServiceFromCommand(cmd)
			if err != nil {
				return err
			}
			defer cleanup()

			limit, _ := cmd.Flags().GetInt64("limit")
			updated, err := svc.BackfillEmbeddings(context.Background(), limit)
			if err != nil {
				return err
			}
			fmt.Printf("Backfilled embeddings: %d\n", updated)
			return nil
		},
	}
	ragCmd.AddCommand(backfillCmd)

	backfillCmd.Flags().Bool("debug", false, "Debug")
	backfillCmd.Flags().String("cwd", "", "Current working directory")
	backfillCmd.Flags().Int64("limit", 0, "Maximum number of chunks to backfill (0 = all)")

	ragSearchCmd.Flags().String("query", "", "Search query")
	ragSearchCmd.Flags().Int64("limit", 5, "Result limit")

	ragBenchmarkCmd.Flags().String("query", "", "Benchmark query")
	ragBenchmarkCmd.Flags().Int64("limit", 5, "Result limit")
	ragBenchmarkCmd.Flags().Int("runs", 10, "Number of benchmark runs")
	ragBenchmarkCmd.Flags().String("expected-term", "", "Optional term for proxy quality scoring")

	_ = ragIndexCmd.MarkFlagRequired("uri")
}
