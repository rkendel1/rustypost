package scanner

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"flux/internal/models"
	"flux/internal/schema"
)

type Parameter struct {
	Name     string `json:"name"`
	In       string `json:"in"`
	Required bool   `json:"required"`
}

type Endpoint struct {
	Method          string                    `json:"method"`
	Path            string                    `json:"path"`
	SourceFiles     []string                  `json:"sourceFiles"`
	LineNumbers     []int                     `json:"lineNumbers"`
	AuthSchemes     []string                  `json:"authSchemes,omitempty"`
	Parameters      []Parameter               `json:"parameters,omitempty"`
	RequestSchema   map[string]any            `json:"requestSchema,omitempty"`
	ResponseSchemas map[string]map[string]any `json:"responseSchemas,omitempty"`
}

type Inventory struct {
	GeneratedAt  string     `json:"generatedAt"`
	Repository   string     `json:"repository"`
	FilesScanned int        `json:"filesScanned"`
	Endpoints    []Endpoint `json:"endpoints"`
	AuthSchemes  []string   `json:"authSchemes"`
}

type Artifacts struct {
	OutputDir     string `json:"outputDir"`
	OpenAPIPath   string `json:"openapiPath"`
	WorkspacePath string `json:"workspacePath"`
	InventoryPath string `json:"inventoryPath"`
	HarnessPath   string `json:"harnessPath"`
	DriftPath     string `json:"driftPath"`
}

type routePattern struct {
	re            *regexp.Regexp
	methodIdx     int
	pathIdx       int
	defaultMethod string
}

var routePatterns = []routePattern{
	{re: regexp.MustCompile("(?i)\\b(?:app|router|r|e|mux)\\.(get|post|put|patch|delete|options|head)\\s*\\(\\s*[\"'`](/[^\"'`]*)[\"'`]"), methodIdx: 1, pathIdx: 2},
	{re: regexp.MustCompile("(?i)\\b(?:Method|Handle)\\s*\\(\\s*[\"'`](GET|POST|PUT|PATCH|DELETE|OPTIONS|HEAD)[\"'`]\\s*,\\s*[\"'`](/[^\"'`]*)[\"'`]"), methodIdx: 1, pathIdx: 2},
	{re: regexp.MustCompile(`(?im)@route\s+(GET|POST|PUT|PATCH|DELETE|OPTIONS|HEAD)\s+(/[\S]*)`), methodIdx: 1, pathIdx: 2},
	{re: regexp.MustCompile("(?i)\\bHandleFunc\\s*\\(\\s*[\"'`](/[^\"'`]*)[\"'`]"), pathIdx: 1, defaultMethod: "GET"},
}

func ScanRepository(root string) (*Inventory, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	stat, err := os.Stat(absRoot)
	if err != nil {
		return nil, err
	}
	if !stat.IsDir() {
		return nil, fmt.Errorf("scan target must be a directory")
	}

	type collector struct {
		method string
		path   string
		files  map[string]struct{}
		lines  map[int]struct{}
		auth   map[string]struct{}
		params map[string]Parameter
		req    map[string]any
		resp   map[string]map[string]any
	}

	collected := map[string]*collector{}
	filesScanned := 0
	globalAuth := map[string]struct{}{}

	err = filepath.WalkDir(absRoot, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		name := d.Name()
		if d.IsDir() {
			switch name {
			case ".git", "node_modules", "vendor", "dist", "build", ".next", ".turbo":
				return filepath.SkipDir
			}
			return nil
		}
		if !isScannableFile(path) {
			return nil
		}
		filesScanned++

		b, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		content := string(b)
		rel, _ := filepath.Rel(absRoot, path)
		rel = filepath.ToSlash(rel)

		for _, rp := range routePatterns {
			matches := rp.re.FindAllStringSubmatchIndex(content, -1)
			for _, idx := range matches {
				method := rp.defaultMethod
				if rp.methodIdx > 0 {
					method = strings.ToUpper(content[idx[2*rp.methodIdx]:idx[2*rp.methodIdx+1]])
				}
				rawPath := content[idx[2*rp.pathIdx]:idx[2*rp.pathIdx+1]]
				normalizedPath := normalizePath(rawPath)
				if normalizedPath == "" {
					continue
				}
				line := lineNumber(content, idx[0])
				key := method + " " + normalizedPath
				entry, ok := collected[key]
				if !ok {
					entry = &collector{
						method: method,
						path:   normalizedPath,
						files:  map[string]struct{}{},
						lines:  map[int]struct{}{},
						auth:   map[string]struct{}{},
						params: map[string]Parameter{},
						req:    inferRequestSchema(method),
						resp:   map[string]map[string]any{},
					}
					for _, p := range extractPathParams(normalizedPath) {
						entry.params[p.Name+":"+p.In] = p
					}
					for _, p := range extractQueryParams(normalizedPath) {
						entry.params[p.Name+":"+p.In] = p
					}
					collected[key] = entry
				}
				entry.files[rel] = struct{}{}
				entry.lines[line] = struct{}{}

				snippet := extractSnippet(content, idx[0], idx[1], 800)
				for _, a := range inferAuthSchemes(snippet) {
					entry.auth[a] = struct{}{}
					globalAuth[a] = struct{}{}
				}
				if len(entry.resp) == 0 {
					entry.resp["200"] = inferResponseSchema(snippet)
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	endpoints := make([]Endpoint, 0, len(collected))
	for _, c := range collected {
		ep := Endpoint{
			Method:          c.method,
			Path:            c.path,
			SourceFiles:     setToSortedStrings(c.files),
			LineNumbers:     setToSortedInts(c.lines),
			AuthSchemes:     setToSortedStrings(c.auth),
			Parameters:      sortedParameters(c.params),
			RequestSchema:   c.req,
			ResponseSchemas: c.resp,
		}
		endpoints = append(endpoints, ep)
	}
	sort.Slice(endpoints, func(i, j int) bool {
		if endpoints[i].Path != endpoints[j].Path {
			return endpoints[i].Path < endpoints[j].Path
		}
		return endpoints[i].Method < endpoints[j].Method
	})

	return &Inventory{
		GeneratedAt:  time.Now().UTC().Format(time.RFC3339),
		Repository:   absRoot,
		FilesScanned: filesScanned,
		Endpoints:    endpoints,
		AuthSchemes:  setToSortedStrings(globalAuth),
	}, nil
}

func GenerateArtifacts(root, outDir string, inv *Inventory) (*Artifacts, error) {
	if inv == nil {
		return nil, fmt.Errorf("inventory is required")
	}
	if outDir == "" {
		outDir = filepath.Join(root, ".reqit", "scan")
	}
	if err := os.MkdirAll(filepath.Join(outDir, "tests"), 0o755); err != nil {
		return nil, err
	}

	openapiPath := filepath.Join(outDir, "openapi.json")
	workspacePath := filepath.Join(outDir, "workspace.json")
	inventoryPath := filepath.Join(outDir, "inventory.json")
	harnessPath := filepath.Join(outDir, "tests", "scan-harness.js")
	driftPath := filepath.Join(outDir, "drift.json")

	var oldSpecPath string
	if b, err := os.ReadFile(openapiPath); err == nil {
		oldSpecPath = filepath.Join(outDir, ".openapi.previous.json")
		if writeErr := os.WriteFile(oldSpecPath, b, 0o644); writeErr == nil {
			defer os.Remove(oldSpecPath)
		} else {
			oldSpecPath = ""
		}
	}

	if err := writeJSON(openapiPath, buildOpenAPI(inv)); err != nil {
		return nil, err
	}
	if err := writeJSON(workspacePath, buildWorkspace(inv)); err != nil {
		return nil, err
	}
	if err := writeJSON(inventoryPath, inv); err != nil {
		return nil, err
	}
	if err := os.WriteFile(harnessPath, []byte(buildHarness(inv)), 0o755); err != nil {
		return nil, err
	}
	if err := writeJSON(driftPath, buildDrift(oldSpecPath, openapiPath)); err != nil {
		return nil, err
	}

	return &Artifacts{
		OutputDir:     outDir,
		OpenAPIPath:   openapiPath,
		WorkspacePath: workspacePath,
		InventoryPath: inventoryPath,
		HarnessPath:   harnessPath,
		DriftPath:     driftPath,
	}, nil
}

func buildOpenAPI(inv *Inventory) map[string]any {
	paths := map[string]any{}
	components := map[string]any{
		"securitySchemes": map[string]any{},
	}
	securitySchemes := components["securitySchemes"].(map[string]any)

	for _, ep := range inv.Endpoints {
		p, ok := paths[ep.Path]
		if !ok {
			p = map[string]any{}
			paths[ep.Path] = p
		}
		pi := p.(map[string]any)
		op := map[string]any{
			"operationId": operationID(ep),
			"responses":   map[string]any{},
			"tags":        []string{"discovered"},
		}
		if len(ep.Parameters) > 0 {
			params := make([]map[string]any, 0, len(ep.Parameters))
			for _, prm := range ep.Parameters {
				params = append(params, map[string]any{
					"name":     prm.Name,
					"in":       prm.In,
					"required": prm.Required,
					"schema": map[string]any{
						"type": "string",
					},
				})
			}
			op["parameters"] = params
		}
		if len(ep.RequestSchema) > 0 && ep.Method != "GET" && ep.Method != "HEAD" && ep.Method != "OPTIONS" {
			op["requestBody"] = map[string]any{
				"required": false,
				"content": map[string]any{
					"application/json": map[string]any{
						"schema": ep.RequestSchema,
					},
				},
			}
		}
		responses := op["responses"].(map[string]any)
		for code, schemaRef := range ep.ResponseSchemas {
			responses[code] = map[string]any{
				"description": "Discovered response",
				"content": map[string]any{
					"application/json": map[string]any{
						"schema": schemaRef,
					},
				},
			}
		}
		if len(responses) == 0 {
			responses["200"] = map[string]any{"description": "Discovered response"}
		}
		if len(ep.AuthSchemes) > 0 {
			reqs := []map[string][]string{}
			for _, a := range ep.AuthSchemes {
				switch a {
				case "bearer":
					securitySchemes["bearer"] = map[string]any{"type": "http", "scheme": "bearer"}
				case "basic":
					securitySchemes["basic"] = map[string]any{"type": "http", "scheme": "basic"}
				case "apiKey":
					securitySchemes["apiKey"] = map[string]any{"type": "apiKey", "in": "header", "name": "X-API-Key"}
				case "oauth2":
					securitySchemes["oauth2"] = map[string]any{
						"type": "oauth2",
						"flows": map[string]any{
							"clientCredentials": map[string]any{
								"tokenUrl": "https://example.com/oauth/token",
								"scopes":   map[string]string{},
							},
						},
					}
				}
				reqs = append(reqs, map[string][]string{a: []string{}})
			}
			op["security"] = reqs
		}
		pi[strings.ToLower(ep.Method)] = op
	}

	return map[string]any{
		"openapi":           "3.1.0",
		"jsonSchemaDialect": "https://json-schema.org/draft/2020-12/schema",
		"info": map[string]any{
			"title":   "Discovered API",
			"version": "1.0.0",
		},
		"servers": []map[string]any{
			{"url": "{{BASE_URL}}"},
		},
		"paths":         paths,
		"components":    components,
		"x-generatedBy": "reqit scan",
	}
}

func buildWorkspace(inv *Inventory) map[string]any {
	requests := make([]models.SavedRequest, 0, len(inv.Endpoints))
	collID := uuid.NewString()
	now := time.Now().UTC().Format(time.RFC3339)

	for _, ep := range inv.Endpoints {
		payload := models.RequestPayload{
			Method:   ep.Method,
			URL:      "{{BASE_URL}}" + ep.Path,
			BodyType: "none",
		}
		if ep.Method != "GET" && ep.Method != "HEAD" && ep.Method != "OPTIONS" {
			payload.BodyType = "json"
			payload.Body = "{}"
		}
		if len(ep.AuthSchemes) > 0 {
			payload.AuthType = ep.AuthSchemes[0]
			switch payload.AuthType {
			case "bearer":
				payload.AuthValue = "{{TOKEN}}"
			case "apiKey":
				payload.AuthValue = "header:X-API-Key:{{API_KEY}}"
			}
		}
		requests = append(requests, models.SavedRequest{
			ID:        uuid.NewString(),
			Name:      fmt.Sprintf("%s %s", ep.Method, ep.Path),
			CollID:    collID,
			Payload:   payload,
			CreatedAt: now,
		})
	}

	return map[string]any{
		"version":     "1",
		"generatedAt": time.Now().UTC().Format(time.RFC3339),
		"collections": []models.Collection{
			{
				ID:       collID,
				Name:     "Discovered API",
				Requests: requests,
			},
		},
		"environments": []models.Environment{
			{
				ID:   uuid.NewString(),
				Name: "local",
				Vars: []models.EnvVar{
					{Key: "BASE_URL", Value: "http://localhost:3000", Enabled: true},
					{Key: "TOKEN", Value: "", Enabled: true},
					{Key: "API_KEY", Value: "", Enabled: true},
				},
			},
		},
	}
}

func buildHarness(inv *Inventory) string {
	payload, _ := json.Marshal(inv.Endpoints)
	return `#!/usr/bin/env node
const baseUrl = process.env.REQIT_SCAN_BASE_URL || process.env.BASE_URL;
const endpoints = ` + string(payload) + `;

if (!baseUrl) {
  console.error("REQIT_SCAN_BASE_URL (or BASE_URL) is required.");
  process.exit(1);
}

async function run() {
  let failed = 0;
  for (const ep of endpoints) {
    const url = new URL(ep.path, baseUrl).toString();
    const method = ep.method || "GET";
    const body = (method === "GET" || method === "HEAD" || method === "OPTIONS") ? undefined : "{}";
    try {
      const resp = await fetch(url, {
        method,
        headers: { "content-type": "application/json" },
        body
      });
      if (resp.status >= 500) {
        failed++;
        console.error("FAIL", method, url, resp.status);
      } else {
        console.log("PASS", method, url, resp.status);
      }
    } catch (e) {
      failed++;
      console.error("FAIL", method, url, e.message);
    }
  }
  process.exit(failed === 0 ? 0 : 1);
}

run();
`
}

func buildDrift(oldSpecPath, newSpecPath string) map[string]any {
	if oldSpecPath == "" {
		return map[string]any{
			"summary": "No previous scan output found. Baseline snapshot created.",
			"changes": 0,
		}
	}
	oldSnap, oldErr := schema.CaptureSnapshot(oldSpecPath)
	newSnap, newErr := schema.CaptureSnapshot(newSpecPath)
	if oldErr != nil || newErr != nil {
		return map[string]any{
			"summary": fmt.Sprintf("Unable to detect drift: old=%v new=%v", oldErr, newErr),
			"changes": 0,
		}
	}
	drift := schema.DetectDrift(oldSnap, newSnap)
	return map[string]any{
		"summary":  drift.Summary,
		"added":    drift.Added,
		"removed":  drift.Removed,
		"changed":  drift.Changed,
		"breaking": drift.BreakingChanges(),
		"changes":  len(drift.Added) + len(drift.Removed) + len(drift.Changed),
	}
}

func operationID(ep Endpoint) string {
	r := strings.NewReplacer("/", "_", "{", "", "}", "", "-", "_")
	return strings.ToLower(ep.Method) + "_" + strings.Trim(r.Replace(ep.Path), "_")
}

func inferAuthSchemes(s string) []string {
	l := strings.ToLower(s)
	out := map[string]struct{}{}
	if strings.Contains(l, "bearer") || strings.Contains(l, "jwt") || strings.Contains(l, "authorization") {
		out["bearer"] = struct{}{}
	}
	if strings.Contains(l, "basic auth") || strings.Contains(l, "authorization: basic") {
		out["basic"] = struct{}{}
	}
	if strings.Contains(l, "x-api-key") || strings.Contains(l, "api key") || strings.Contains(l, "apikey") {
		out["apiKey"] = struct{}{}
	}
	if strings.Contains(l, "oauth") {
		out["oauth2"] = struct{}{}
	}
	return setToSortedStrings(out)
}

func inferRequestSchema(method string) map[string]any {
	if method == "GET" || method == "HEAD" || method == "OPTIONS" {
		return nil
	}
	return map[string]any{
		"type":                 "object",
		"additionalProperties": true,
	}
}

func inferResponseSchema(snippet string) map[string]any {
	keys := inferJSONKeys(snippet)
	props := map[string]any{}
	required := make([]string, 0, len(keys))
	for _, k := range keys {
		props[k] = map[string]any{"type": "string"}
		required = append(required, k)
	}
	if len(props) == 0 {
		return map[string]any{
			"type":                 "object",
			"additionalProperties": true,
		}
	}
	return map[string]any{
		"type":       "object",
		"properties": props,
		"required":   required,
	}
}

func inferJSONKeys(snippet string) []string {
	obj := ""
	if m := regexp.MustCompile(`(?s)res\.json\s*\(\s*(\{.*?\})\s*\)`).FindStringSubmatch(snippet); len(m) > 1 {
		obj = m[1]
	} else if m := regexp.MustCompile(`(?s)JSON\s*\(\s*\d+\s*,\s*(\{.*?\})\s*\)`).FindStringSubmatch(snippet); len(m) > 1 {
		obj = m[1]
	}
	if obj == "" {
		return nil
	}
	keyRx := regexp.MustCompile(`["']?([A-Za-z_][A-Za-z0-9_]*)["']?\s*:`)
	matches := keyRx.FindAllStringSubmatch(obj, -1)
	set := map[string]struct{}{}
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		set[m[1]] = struct{}{}
	}
	return setToSortedStrings(set)
}

func extractPathParams(path string) []Parameter {
	rx := regexp.MustCompile(`\{([A-Za-z_][A-Za-z0-9_]*)\}`)
	matches := rx.FindAllStringSubmatch(path, -1)
	var out []Parameter
	for _, m := range matches {
		out = append(out, Parameter{Name: m[1], In: "path", Required: true})
	}
	return out
}

func extractQueryParams(path string) []Parameter {
	i := strings.Index(path, "?")
	if i == -1 || i+1 >= len(path) {
		return nil
	}
	q := path[i+1:]
	parts := strings.Split(q, "&")
	var out []Parameter
	for _, p := range parts {
		if p == "" {
			continue
		}
		name := strings.SplitN(p, "=", 2)[0]
		name = strings.TrimSpace(name)
		if name != "" {
			out = append(out, Parameter{Name: name, In: "query"})
		}
	}
	return out
}

func normalizePath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	p = regexp.MustCompile(`:([A-Za-z_][A-Za-z0-9_]*)`).ReplaceAllString(p, `{$1}`)
	pathOnly := strings.SplitN(p, " ", 2)[0]
	return pathOnly
}

func extractSnippet(content string, start, end, window int) string {
	lo := start - window
	if lo < 0 {
		lo = 0
	}
	hi := end + window
	if hi > len(content) {
		hi = len(content)
	}
	return content[lo:hi]
}

func lineNumber(s string, idx int) int {
	if idx < 0 {
		return 1
	}
	if idx > len(s) {
		idx = len(s)
	}
	return strings.Count(s[:idx], "\n") + 1
}

func isScannableFile(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go", ".js", ".jsx", ".ts", ".tsx", ".py", ".rb", ".java", ".kt", ".php", ".cs":
		return true
	default:
		return false
	}
}

func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

func setToSortedStrings(m map[string]struct{}) []string {
	if len(m) == 0 {
		return nil
	}
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func setToSortedInts(m map[int]struct{}) []int {
	if len(m) == 0 {
		return nil
	}
	out := make([]int, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Ints(out)
	return out
}

func sortedParameters(m map[string]Parameter) []Parameter {
	if len(m) == 0 {
		return nil
	}
	out := make([]Parameter, 0, len(m))
	for _, p := range m {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].In != out[j].In {
			return out[i].In < out[j].In
		}
		return out[i].Name < out[j].Name
	})
	return out
}
