package uiapp

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"riftsync/internal/serverapp"
)

func TestAppProtectionEndpoints(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "riftsync-ui-prot-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// Create test script
	modDir := filepath.Join(tempDir, "ReplicatedStorage", "Combat")
	if err := os.MkdirAll(modDir, 0755); err != nil {
		t.Fatal(err)
	}
	scriptPath := filepath.Join(modDir, "Attack.module.luau")
	if err := os.WriteFile(scriptPath, []byte("return { damage = 100 }"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create sync_config.json
	cfgPath := filepath.Join(tempDir, "sync_config.json")
	if err := os.WriteFile(cfgPath, []byte(`{"sync_root":"."}`), 0644); err != nil {
		t.Fatal(err)
	}

	controller := &appController{
		runner:  serverapp.New(serverapp.Options{ConfigPath: cfgPath, SyncRootOverride: tempDir, PortOverride: -1}),
		options: serverapp.Options{ConfigPath: cfgPath, SyncRootOverride: tempDir, PortOverride: -1},
		quit:    make(chan struct{}),
	}
	mux := http.NewServeMux()
	controller.registerRoutes(mux)

	// 1. GET /app/protection/tree
	req := httptest.NewRequest(http.MethodGet, "/app/protection/tree", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Attack.module.luau") {
		t.Fatalf("tree failed: %d %s", rec.Code, rec.Body.String())
	}

	// 2. POST /app/protection/protect
	relPath := "ReplicatedStorage/Combat/Attack.module.luau"
	protReq := httptest.NewRequest(http.MethodPost, "/app/protection/protect",
		bytes.NewBufferString(`{"paths":["`+relPath+`"],"watermark":"TestWatermark"}`))
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, protReq)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"processed"`) {
		t.Fatalf("protect failed: %d %s", rec.Code, rec.Body.String())
	}

	// Verify file on disk is obfuscated
	content, _ := os.ReadFile(scriptPath)
	if !strings.Contains(string(content), "TestWatermark") {
		t.Fatalf("script not obfuscated: %s", string(content))
	}

	// 3. POST /app/protection/restore
	restReq := httptest.NewRequest(http.MethodPost, "/app/protection/restore",
		bytes.NewBufferString(`{"paths":["`+relPath+`"]}`))
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, restReq)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"restored"`) {
		t.Fatalf("restore failed: %d %s", rec.Code, rec.Body.String())
	}

	// Verify file restored
	restored, _ := os.ReadFile(scriptPath)
	if string(restored) != "return { damage = 100 }" {
		t.Fatalf("script not restored correctly: %s", string(restored))
	}
}

func TestAppExplorerEndpoints(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "riftsync-ui-exp-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	serverDir := filepath.Join(tempDir, "ServerScriptService")
	if err := os.MkdirAll(serverDir, 0755); err != nil {
		t.Fatal(err)
	}
	scriptPath := filepath.Join(serverDir, "Main.server.luau")
	if err := os.WriteFile(scriptPath, []byte("print('hello')"), 0644); err != nil {
		t.Fatal(err)
	}

	cfgPath := filepath.Join(tempDir, "sync_config.json")
	if err := os.WriteFile(cfgPath, []byte(`{"sync_root":"."}`), 0644); err != nil {
		t.Fatal(err)
	}

	controller := &appController{
		runner:  serverapp.New(serverapp.Options{ConfigPath: cfgPath, SyncRootOverride: tempDir, PortOverride: -1}),
		options: serverapp.Options{ConfigPath: cfgPath, SyncRootOverride: tempDir, PortOverride: -1},
		quit:    make(chan struct{}),
	}
	mux := http.NewServeMux()
	controller.registerRoutes(mux)

	// 1. GET /app/explorer/tree
	req := httptest.NewRequest(http.MethodGet, "/app/explorer/tree", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Main.server.luau") {
		t.Fatalf("explorer tree failed: %d %s", rec.Code, rec.Body.String())
	}

	// 2. POST /app/explorer/set-enabled (disable)
	relPath := "ServerScriptService/Main.server.luau"
	disReq := httptest.NewRequest(http.MethodPost, "/app/explorer/set-enabled",
		bytes.NewBufferString(`{"paths":["`+relPath+`"],"enabled":false}`))
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, disReq)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"updated"`) {
		t.Fatalf("set-enabled false failed: %d %s", rec.Code, rec.Body.String())
	}

	// Verify properties.init.json created and has Enabled = false
	propPath := filepath.Join(serverDir, "Main.Script", "properties.init.json")
	pData, err := os.ReadFile(propPath)
	if err != nil {
		t.Fatalf("properties.init.json missing: %v", err)
	}
	if !strings.Contains(string(pData), `"Enabled": false`) && !strings.Contains(string(pData), `"Enabled":false`) {
		t.Fatalf("properties.init.json unexpected: %s", string(pData))
	}

	// 3. GET /app/explorer/tree should now reflect DisabledScripts = 1
	req = httptest.NewRequest(http.MethodGet, "/app/explorer/tree", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if !strings.Contains(rec.Body.String(), `"disabled_scripts":1`) && !strings.Contains(rec.Body.String(), `"disabled_scripts": 1`) {
		t.Fatalf("explorer tree did not report disabled_scripts: %s", rec.Body.String())
	}

	// 4. POST /app/explorer/set-enabled (re-enable)
	enReq := httptest.NewRequest(http.MethodPost, "/app/explorer/set-enabled",
		bytes.NewBufferString(`{"paths":["ServerScriptService/Main.Script/Main.server.luau"],"enabled":true}`))
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, enReq)
	if rec.Code != http.StatusOK {
		t.Fatalf("set-enabled true failed: %d %s", rec.Code, rec.Body.String())
	}

	pData2, _ := os.ReadFile(propPath)
	if strings.Contains(string(pData2), `"Enabled": false`) || strings.Contains(string(pData2), `"Enabled":false`) {
		t.Fatalf("properties.init.json still disabled: %s", string(pData2))
	}

	// 5. POST /app/explorer/set-properties (edit custom property)
	propReq := httptest.NewRequest(http.MethodPost, "/app/explorer/set-properties",
		bytes.NewBufferString(`{"path":"ServerScriptService/Main.Script","properties":{"RunContext":"Server","CustomVal":42}}`))
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, propReq)
	if rec.Code != http.StatusOK {
		t.Fatalf("set-properties failed: %d %s", rec.Code, rec.Body.String())
	}

	pData3, _ := os.ReadFile(propPath)
	if !strings.Contains(string(pData3), `"RunContext": "Server"`) && !strings.Contains(string(pData3), `"RunContext":"Server"`) {
		t.Fatalf("properties.init.json missing RunContext: %s", string(pData3))
	}

	// 6. POST /app/explorer/rename
	renameReq := httptest.NewRequest(http.MethodPost, "/app/explorer/rename",
		bytes.NewBufferString(`{"path":"ServerScriptService/Main.Script","new_name":"RenamedScript"}`))
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, renameReq)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"new_path"`) {
		t.Fatalf("rename failed: %d %s", rec.Code, rec.Body.String())
	}
	renamedDir := filepath.Join(serverDir, "RenamedScript.Script")
	if _, err := os.Stat(renamedDir); os.IsNotExist(err) {
		t.Fatalf("renamed folder missing on disk: %s", renamedDir)
	}

	// 7. POST /app/explorer/delete
	delReq := httptest.NewRequest(http.MethodPost, "/app/explorer/delete",
		bytes.NewBufferString(`{"path":"ServerScriptService/RenamedScript.Script"}`))
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, delReq)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete failed: %d %s", rec.Code, rec.Body.String())
	}
	if _, err := os.Stat(renamedDir); !os.IsNotExist(err) {
		t.Fatalf("deleted folder still exists: %s", renamedDir)
	}
}

