package main_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// setupHome は一時ディレクトリに ~/.claude/history.jsonl を作成し、
// HOME パスを返す。
func setupHome(t *testing.T, lines []string) string {
	t.Helper()
	home := t.TempDir()
	claudeDir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatalf("setupHome: mkdir .claude: %v", err)
	}
	content := strings.Join(lines, "\n")
	if err := os.WriteFile(filepath.Join(claudeDir, "history.jsonl"), []byte(content), 0o644); err != nil {
		t.Fatalf("setupHome: write history.jsonl: %v", err)
	}
	return home
}

// runMain は HOME を設定してバイナリを go run . で実行し、
// stdout・stderr・終了コードを返す。
// ANTHROPIC_API_KEY は意図的に環境変数から除外する。
func runMain(t *testing.T, home string) (stdout, stderr string, exitCode int) {
	t.Helper()
	cmd := exec.Command("go", "run", ".")
	cmd.Dir = "."

	// ANTHROPIC_API_KEY を除外した環境変数を構築する
	var env []string
	for _, e := range os.Environ() {
		if !strings.HasPrefix(e, "ANTHROPIC_API_KEY=") {
			env = append(env, e)
		}
	}
	env = append(env, "HOME="+home)
	cmd.Env = env

	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	exitCode = 0
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("runMain: unexpected exec error: %v", err)
		}
	}
	return outBuf.String(), errBuf.String(), exitCode
}

// longLine は display が15文字以上の有効な JSONL エントリを返す。
func longLine(display string) string {
	return fmt.Sprintf(`{"display":%q,"pastedContents":{},"timestamp":1000,"project":"/foo","sessionId":"s1"}`, display)
}

// TestMain_OutputsNumberedLines はプロンプトが "N. テキスト" 形式で出力されることを確認する。
func TestMain_OutputsNumberedLines(t *testing.T) {
	// Given: 有効なプロンプト3件
	lines := []string{
		longLine("これは15文字以上のテストプロンプトA"),
		longLine("これは15文字以上のテストプロンプトB"),
		longLine("これは15文字以上のテストプロンプトC"),
	}
	home := setupHome(t, lines)

	// When
	stdout, _, exitCode := runMain(t, home)

	// Then: 終了コード0
	if exitCode != 0 {
		t.Fatalf("expected exit 0, got %d", exitCode)
	}

	// Then: 出力行数が3（各行 "N. テキスト" 形式）
	outputLines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	if len(outputLines) != 3 {
		t.Fatalf("expected 3 output lines, got %d: %q", len(outputLines), stdout)
	}

	// Then: 各行が "N. " で始まる番号付き形式
	for i, line := range outputLines {
		expected := fmt.Sprintf("%d. ", i+1)
		if !strings.HasPrefix(line, expected) {
			t.Errorf("line %d: expected prefix %q, got %q", i+1, expected, line)
		}
	}
}

// TestMain_OutputsFullTextNotDisplayOnly は pastedContents のテキストが
// FullText として出力に含まれることを確認する。
func TestMain_OutputsFullTextNotDisplayOnly(t *testing.T) {
	// Given: display + pastedContents を持つエントリ
	line := `{"display":"これは15文字以上のテストプロンプトです","pastedContents":{"1":{"id":1,"type":"text","content":"pasted unique content xyz"}},"timestamp":1000,"project":"/foo","sessionId":"s1"}`
	home := setupHome(t, []string{line})

	// When
	stdout, _, exitCode := runMain(t, home)

	// Then: 終了コード0
	if exitCode != 0 {
		t.Fatalf("expected exit 0, got %d", exitCode)
	}

	// Then: pasted content が stdout に含まれる（FullText を使用している証拠）
	if !strings.Contains(stdout, "pasted unique content xyz") {
		t.Errorf("expected pasted content in stdout, got: %q", stdout)
	}
}

// TestMain_SucceedsWithoutAPIKey は ANTHROPIC_API_KEY が未設定でも
// 正常終了することを確認する（旧仕様では exit 1 だった）。
func TestMain_SucceedsWithoutAPIKey(t *testing.T) {
	// Given: ANTHROPIC_API_KEY なし・有効な履歴あり
	lines := []string{longLine("これは15文字以上のテストプロンプトです")}
	home := setupHome(t, lines)

	// When: runMain は ANTHROPIC_API_KEY を除外して実行する
	_, stderr, exitCode := runMain(t, home)

	// Then: API キーなしでも exit 0
	if exitCode != 0 {
		t.Fatalf("expected exit 0 without ANTHROPIC_API_KEY, got %d; stderr: %q", exitCode, stderr)
	}
}

// TestMain_ExitsOneOnMissingHistoryFile はローカル履歴が存在しない場合に
// exit 1 かつ stderr にエラーメッセージが出力されることを確認する。
func TestMain_ExitsOneOnMissingHistoryFile(t *testing.T) {
	// Given: history.jsonl が存在しない HOME
	home := t.TempDir()
	// .claude ディレクトリだけ作成（history.jsonl はなし）
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatalf("mkdir .claude: %v", err)
	}

	// When
	_, stderr, exitCode := runMain(t, home)

	// Then: exit 1
	if exitCode != 1 {
		t.Fatalf("expected exit 1 for missing history file, got %d", exitCode)
	}

	// Then: stderr にエラーメッセージあり
	if strings.TrimSpace(stderr) == "" {
		t.Error("expected error message in stderr, got empty")
	}
}

// TestMain_ExitsOneWhenAllPromptsFiltered は全エントリがフィルタされた場合に
// exit 1 となることを確認する。
func TestMain_ExitsOneWhenAllPromptsFiltered(t *testing.T) {
	// Given: display が全て14文字以下かつ pastedContents 空（全てフィルタされる）
	lines := []string{
		`{"display":"短い","pastedContents":{},"timestamp":1000,"project":"/foo","sessionId":"s1"}`,
		`{"display":"!","pastedContents":{},"timestamp":1001,"project":"/foo","sessionId":"s1"}`,
	}
	home := setupHome(t, lines)

	// When
	_, stderr, exitCode := runMain(t, home)

	// Then: exit 1
	if exitCode != 1 {
		t.Fatalf("expected exit 1 when all entries filtered, got %d", exitCode)
	}

	// Then: stderr にエラーメッセージあり
	if strings.TrimSpace(stderr) == "" {
		t.Error("expected error message in stderr, got empty")
	}
}

// TestMain_LimitsOutputToTen は有効エントリが10件超でも出力が10行以下になることを確認する。
func TestMain_LimitsOutputToTen(t *testing.T) {
	// Given: 有効エントリ15件
	lines := make([]string, 15)
	for i := range lines {
		lines[i] = longLine(fmt.Sprintf("これは15文字以上のテストプロンプト番号%03d", i))
	}
	home := setupHome(t, lines)

	// When
	stdout, _, exitCode := runMain(t, home)

	// Then: 終了コード0
	if exitCode != 0 {
		t.Fatalf("expected exit 0, got %d", exitCode)
	}

	// Then: 出力行数が10（limit）
	outputLines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	if len(outputLines) != 10 {
		t.Errorf("expected 10 output lines (limit), got %d", len(outputLines))
	}
}
