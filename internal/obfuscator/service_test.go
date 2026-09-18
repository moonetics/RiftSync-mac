package obfuscator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestObfuscatorService(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "riftsync-obf-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// Create Roblox hierarchy
	sssDir := filepath.Join(tempDir, "ServerScriptService", "Combat.Script")
	if err := os.MkdirAll(sssDir, 0755); err != nil {
		t.Fatal(err)
	}
	sampleScript := filepath.Join(sssDir, "DamageCalc.module.luau")
	originalCode := `local M = {}
function M.calc(a: number, b: number)
    return a * b + 10
end
return M`
	if err := os.WriteFile(sampleScript, []byte(originalCode), 0644); err != nil {
		t.Fatal(err)
	}

	// 1. Build Explorer Tree
	summary, err := BuildExplorer(tempDir)
	if err != nil {
		t.Fatalf("BuildExplorer failed: %v", err)
	}
	if summary.TotalScripts != 1 || summary.ProtectedScripts != 0 || summary.OriginalScripts != 1 {
		t.Fatalf("summary unexpected: %+v", summary)
	}

	relPath := "ServerScriptService/Combat.Script/DamageCalc.module.luau"

	// 2. Protect script
	protected, err := ProtectScripts(tempDir, []string{relPath}, "Protected by RiftSync")
	if err != nil {
		t.Fatalf("ProtectScripts failed: %v", err)
	}
	if len(protected) != 1 || protected[0] != relPath {
		t.Fatalf("unexpected protected list: %v", protected)
	}

	// Verify file content is obfuscated
	obfBytes, err := os.ReadFile(sampleScript)
	if err != nil {
		t.Fatal(err)
	}
	obfStr := string(obfBytes)
	if !strings.Contains(obfStr, "Protected by RiftSync") || !strings.Contains(obfStr, "S O R E V I U M") {
		t.Fatalf("file not obfuscated: %s", obfStr)
	}

	// Verify vault saved original
	vSummary, err := BuildExplorer(tempDir)
	if err != nil {
		t.Fatal(err)
	}
	if vSummary.ProtectedScripts != 1 || vSummary.OriginalScripts != 0 {
		t.Fatalf("summary should show 1 protected: %+v", vSummary)
	}

	// 3. Restore script
	restored, err := RestoreScripts(tempDir, []string{relPath})
	if err != nil {
		t.Fatalf("RestoreScripts failed: %v", err)
	}
	if len(restored) != 1 || restored[0] != relPath {
		t.Fatalf("unexpected restored list: %v", restored)
	}

	// Verify restored content is equal to original
	restoredBytes, err := os.ReadFile(sampleScript)
	if err != nil {
		t.Fatal(err)
	}
	if string(restoredBytes) != originalCode {
		t.Fatalf("restored code mismatch: %s", string(restoredBytes))
	}
}

func TestSetInstancePropertiesAndUIExplorer(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "riftsync-ui-props-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// Create StarterGui/Shop.ScreenGui/Container.Frame
	frameDir := filepath.Join(tempDir, "StarterGui", "Shop.ScreenGui", "Container.Frame")
	if err := os.MkdirAll(frameDir, 0755); err != nil {
		t.Fatal(err)
	}

	// 1. SetInstanceProperties creates properties.init.json
	err = SetInstanceProperties(tempDir, "StarterGui/Shop.ScreenGui/Container.Frame", map[string]any{
		"BackgroundColor3": "#ff5500",
		"Visible":          true,
	})
	if err != nil {
		t.Fatalf("SetInstanceProperties failed: %v", err)
	}

	// Verify file on disk
	pPath := filepath.Join(frameDir, "properties.init.json")
	pData, err := os.ReadFile(pPath)
	if err != nil {
		t.Fatalf("read properties.init.json failed: %v", err)
	}
	if !strings.Contains(string(pData), "#ff5500") || !strings.Contains(string(pData), `"Visible": true`) && !strings.Contains(string(pData), `"Visible":true`) {
		t.Fatalf("unexpected properties content: %s", string(pData))
	}

	// 2. BuildExplorer finds the UI instances
	summary, err := BuildExplorer(tempDir)
	if err != nil {
		t.Fatalf("BuildExplorer failed: %v", err)
	}
	if summary.TotalInstances < 3 {
		t.Fatalf("expected at least 3 instances (StarterGui, Shop, Container), got %d", summary.TotalInstances)
	}

	// Find Container node
	var findNode func(n *ExplorerNode, name string) *ExplorerNode
	findNode = func(n *ExplorerNode, name string) *ExplorerNode {
		if n.Name == name {
			return n
		}
		for _, c := range n.Children {
			if res := findNode(c, name); res != nil {
				return res
			}
		}
		return nil
	}

	container := findNode(summary.Tree, "Container")
	if container == nil {
		t.Fatalf("Container node not found in tree")
	}
	if container.ClassName != "Frame" || container.Kind != "frame" {
		t.Fatalf("unexpected Container node: %+v", container)
	}
	if container.Properties["BackgroundColor3"] != "#ff5500" {
		t.Fatalf("unexpected BackgroundColor3: %v", container.Properties["BackgroundColor3"])
	}
}

func TestImperialTree(t *testing.T) {
	summary, err := BuildExplorer("/Users/percayajanji/Documents/rblxexperiences/imperial")
	if err != nil {
		t.Skipf("imperial experience not present: %v", err)
		return
	}
	var soundService *ExplorerNode
	for _, c := range summary.Tree.Children {
		if c.Name == "SoundService" {
			soundService = c
			break
		}
	}
	if soundService == nil {
		t.Fatalf("expected SoundService to be a root child of imperial")
	}
	if soundService.Kind != "service" || soundService.ClassName != "SoundService" {
		t.Fatalf("unexpected SoundService properties: %+v", soundService)
	}
}

func TestFolderAndScriptInstanceProtection(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "riftsync-folder-obf-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// Create: ServerScriptService/Leaderboards.Folder/WorkspaceLeaderboardServer.Script/WorkspaceLeaderboardServer.server.luau
	scriptDir := filepath.Join(tempDir, "ServerScriptService", "Leaderboards.Folder", "WorkspaceLeaderboardServer.Script")
	if err := os.MkdirAll(scriptDir, 0755); err != nil {
		t.Fatal(err)
	}
	sampleScript := filepath.Join(scriptDir, "WorkspaceLeaderboardServer.server.luau")
	originalCode := `local Players = game:GetService("Players")
print("Leaderboard running")`
	if err := os.WriteFile(sampleScript, []byte(originalCode), 0644); err != nil {
		t.Fatal(err)
	}

	// 1. Initial explorer
	summary, err := BuildExplorer(tempDir)
	if err != nil {
		t.Fatal(err)
	}
	if summary.TotalScripts != 1 || summary.ProtectedScripts != 0 {
		t.Fatalf("expected 1 total script and 0 protected, got: %+v", summary)
	}

	// 2. Protect passing folder path (as frontend does when folder or instance directory is selected)
	folderRel := "ServerScriptService/Leaderboards.Folder"
	protected, err := ProtectScripts(tempDir, []string{folderRel}, "Protected by RiftSync")
	if err != nil {
		t.Fatalf("ProtectScripts failed: %v", err)
	}
	if len(protected) != 1 {
		t.Fatalf("expected 1 protected file, got %v", protected)
	}

	// 3. BuildExplorer must report script as Protected (VM badge)
	summary2, err := BuildExplorer(tempDir)
	if err != nil {
		t.Fatal(err)
	}
	if summary2.ProtectedScripts != 1 {
		t.Fatalf("expected 1 protected script in summary, got %d", summary2.ProtectedScripts)
	}

	var findScriptNode func(n *ExplorerNode) *ExplorerNode
	findScriptNode = func(n *ExplorerNode) *ExplorerNode {
		if n.IsScript {
			return n
		}
		for _, c := range n.Children {
			if res := findScriptNode(c); res != nil {
				return res
			}
		}
		return nil
	}
	scriptNode := findScriptNode(summary2.Tree)
	if scriptNode == nil {
		t.Fatal("script node not found")
	}
	if !scriptNode.Protected {
		t.Fatalf("script node.Protected should be true, got false")
	}

	// 4. Restore using the script folder path
	instanceRel := "ServerScriptService/Leaderboards.Folder/WorkspaceLeaderboardServer.Script"
	restored, err := RestoreScripts(tempDir, []string{instanceRel})
	if err != nil {
		t.Fatalf("RestoreScripts failed: %v", err)
	}
	if len(restored) != 1 {
		t.Fatalf("expected 1 restored file, got %v", restored)
	}

	// 5. Verify file content restored
	curBytes, err := os.ReadFile(sampleScript)
	if err != nil {
		t.Fatal(err)
	}
	if string(curBytes) != originalCode {
		t.Fatalf("restored code mismatch: %s", string(curBytes))
	}

	// 6. BuildExplorer must now report 0 protected (Orig badge)
	summary3, err := BuildExplorer(tempDir)
	if err != nil {
		t.Fatal(err)
	}
	if summary3.ProtectedScripts != 0 {
		t.Fatalf("expected 0 protected scripts, got %d", summary3.ProtectedScripts)
	}
	scriptNode3 := findScriptNode(summary3.Tree)
	if scriptNode3.Protected {
		t.Fatalf("script node.Protected should be false after restore, got true")
	}
}
