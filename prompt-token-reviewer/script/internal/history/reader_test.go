package history_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"prompt-reviewer/internal/history"
)

// writeHistoryFile は一時ディレクトリに history.jsonl を書き込み、パスを返す。
func writeHistoryFile(t *testing.T, lines []string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "history.jsonl")
	content := strings.Join(lines, "\n")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writeHistoryFile: %v", err)
	}
	return path
}

func writeJSONLFile(t *testing.T, name string, lines []string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	content := strings.Join(lines, "\n")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writeJSONLFile: %v", err)
	}
	return path
}

// TestReadPrompts_ParsesDisplayField は display フィールドが正しくパースされることを確認する。
func TestReadPrompts_ParsesDisplayField(t *testing.T) {
	// Given: display が15文字以上のエントリ1件
	line := `{"display":"これは15文字以上のテストプロンプトです","pastedContents":{},"timestamp":1000,"project":"/foo","sessionId":"s1"}`
	path := writeHistoryFile(t, []string{line})

	// When
	prompts, err := history.ReadPromptsFromFile(path, 10)

	// Then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(prompts) != 1 {
		t.Fatalf("expected 1 prompt, got %d", len(prompts))
	}
	if !strings.Contains(prompts[0].Display, "これは15文字以上") {
		t.Errorf("Display not parsed correctly: %q", prompts[0].Display)
	}
}

// TestReadPrompts_FiltersShortDisplayWithoutPasted は display が14文字以下かつ
// pastedContents が空のエントリを除外することを確認する。
func TestReadPrompts_FiltersShortDisplayWithoutPasted(t *testing.T) {
	// Given: display が14文字以下、pastedContents 空
	line := `{"display":"短いプロンプト","pastedContents":{},"timestamp":1000,"project":"/foo","sessionId":"s1"}`
	path := writeHistoryFile(t, []string{line})

	// When
	_, err := history.ReadPromptsFromFile(path, 10)

	// Then: 有効なプロンプトが0件のため error が返る
	if err == nil {
		t.Fatal("expected error for no meaningful prompts, got nil")
	}
}

// TestReadPrompts_IncludesShortDisplayWithPasted は display が短くても
// pastedContents にテキストがあればフィルタを通過することを確認する。
func TestReadPrompts_IncludesShortDisplayWithPasted(t *testing.T) {
	// Given: display が短い、pastedContents に type=text のコンテンツあり
	line := `{"display":"short","pastedContents":{"1":{"id":1,"type":"text","content":"This is pasted content that is long enough"}},"timestamp":1000,"project":"/foo","sessionId":"s1"}`
	path := writeHistoryFile(t, []string{line})

	// When
	prompts, err := history.ReadPromptsFromFile(path, 10)

	// Then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(prompts) != 1 {
		t.Fatalf("expected 1 prompt, got %d", len(prompts))
	}
}

// TestReadPrompts_AppendsPastedTextToFullText は pastedContents の type=text コンテンツが
// FullText に含まれることを確認する。
func TestReadPrompts_AppendsPastedTextToFullText(t *testing.T) {
	// Given: display と pasted 両方に内容あり
	line := `{"display":"これは15文字以上のテストプロンプトです","pastedContents":{"1":{"id":1,"type":"text","content":"pasted content here"}},"timestamp":1000,"project":"/foo","sessionId":"s1"}`
	path := writeHistoryFile(t, []string{line})

	// When
	prompts, err := history.ReadPromptsFromFile(path, 10)

	// Then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(prompts[0].FullText, "pasted content here") {
		t.Errorf("FullText does not contain pasted content: %q", prompts[0].FullText)
	}
	if !strings.Contains(prompts[0].FullText, "これは15文字以上") {
		t.Errorf("FullText does not contain display: %q", prompts[0].FullText)
	}
}

// TestReadPrompts_IgnoresNonTextPasted は pastedContents で type が "text" 以外のものを
// FullText に含めないことを確認する。
func TestReadPrompts_IgnoresNonTextPasted(t *testing.T) {
	// Given: pastedContents に type=image のみ
	line := `{"display":"これは15文字以上のテストプロンプトです","pastedContents":{"1":{"id":1,"type":"image","content":"base64data"}},"timestamp":1000,"project":"/foo","sessionId":"s1"}`
	path := writeHistoryFile(t, []string{line})

	// When
	prompts, err := history.ReadPromptsFromFile(path, 10)

	// Then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(prompts[0].FullText, "base64data") {
		t.Errorf("FullText should not contain non-text pasted content: %q", prompts[0].FullText)
	}
}

// TestReadPrompts_SamplesUpToLimit は取得件数が limit を超えないことを確認する。
func TestReadPrompts_SamplesUpToLimit(t *testing.T) {
	// Given: 15件の有効エントリ
	lines := make([]string, 15)
	for i := range lines {
		lines[i] = `{"display":"これは15文字以上のテストプロンプトです（番号付き）","pastedContents":{},"timestamp":1000,"project":"/foo","sessionId":"s1"}`
	}
	path := writeHistoryFile(t, lines)

	// When
	prompts, err := history.ReadPromptsFromFile(path, 10)

	// Then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(prompts) != 10 {
		t.Errorf("expected 10 prompts (limit), got %d", len(prompts))
	}
}

// TestReadPrompts_ReturnsAllWhenFewerThanLimit は有効エントリが limit 未満の場合に
// 全件返すことを確認する。
func TestReadPrompts_ReturnsAllWhenFewerThanLimit(t *testing.T) {
	// Given: 5件の有効エントリ
	lines := make([]string, 5)
	for i := range lines {
		lines[i] = `{"display":"これは15文字以上のテストプロンプトです（番号付き）","pastedContents":{},"timestamp":1000,"project":"/foo","sessionId":"s1"}`
	}
	path := writeHistoryFile(t, lines)

	// When
	prompts, err := history.ReadPromptsFromFile(path, 10)

	// Then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(prompts) != 5 {
		t.Errorf("expected 5 prompts (all available), got %d", len(prompts))
	}
}

// TestReadPrompts_ErrorOnEmptyFile は空ファイルでエラーが返ることを確認する。
func TestReadPrompts_ErrorOnEmptyFile(t *testing.T) {
	// Given: 空ファイル
	path := writeHistoryFile(t, []string{})

	// When
	_, err := history.ReadPromptsFromFile(path, 10)

	// Then
	if err == nil {
		t.Fatal("expected error for empty file, got nil")
	}
}

// TestReadPrompts_ErrorWhenAllFiltered は全エントリがフィルタされた場合に
// エラーが返ることを確認する。
func TestReadPrompts_ErrorWhenAllFiltered(t *testing.T) {
	// Given: display が全て14文字以下かつ pastedContents 空
	lines := []string{
		`{"display":"短い","pastedContents":{},"timestamp":1000,"project":"/foo","sessionId":"s1"}`,
		`{"display":"!","pastedContents":{},"timestamp":1001,"project":"/foo","sessionId":"s1"}`,
	}
	path := writeHistoryFile(t, lines)

	// When
	_, err := history.ReadPromptsFromFile(path, 10)

	// Then
	if err == nil {
		t.Fatal("expected error when all entries filtered out, got nil")
	}
}

// TestReadPrompts_SkipsInvalidJSONLines は不正な JSON 行をスキップして処理を続けることを確認する。
func TestReadPrompts_SkipsInvalidJSONLines(t *testing.T) {
	// Given: 不正な行と有効な行が混在
	lines := []string{
		`invalid json line`,
		`{"display":"これは15文字以上のテストプロンプトです","pastedContents":{},"timestamp":1000,"project":"/foo","sessionId":"s1"}`,
	}
	path := writeHistoryFile(t, lines)

	// When
	prompts, err := history.ReadPromptsFromFile(path, 10)

	// Then: 不正行を無視して有効な1件が返る
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(prompts) != 1 {
		t.Errorf("expected 1 valid prompt, got %d", len(prompts))
	}
}

// TestReadPrompts_ExactlyFifteenChars は display が15文字ちょうどで通過することを確認する（境界値）。
func TestReadPrompts_ExactlyFifteenChars(t *testing.T) {
	// Given: display がちょうど15文字（日本語5文字 = 15バイトではなくルーン数）
	// 実装が len(display) >= 15 をルーン数で判定する前提
	// ここでは ASCII 15文字で確認
	line := `{"display":"123456789012345","pastedContents":{},"timestamp":1000,"project":"/foo","sessionId":"s1"}`
	path := writeHistoryFile(t, []string{line})

	// When
	prompts, err := history.ReadPromptsFromFile(path, 10)

	// Then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(prompts) != 1 {
		t.Errorf("expected 1 prompt (exactly 15 chars), got %d", len(prompts))
	}
}

// TestReadPrompts_FourteenCharsFiltered は display が14文字でフィルタされることを確認する（境界値）。
func TestReadPrompts_FourteenCharsFiltered(t *testing.T) {
	// Given: display がちょうど14文字
	line := `{"display":"12345678901234","pastedContents":{},"timestamp":1000,"project":"/foo","sessionId":"s1"}`
	path := writeHistoryFile(t, []string{line})

	// When
	_, err := history.ReadPromptsFromFile(path, 10)

	// Then
	if err == nil {
		t.Fatal("expected error for 14-char display with no pasted, got nil")
	}
}

func TestReadCodexPromptsFromFile_ParsesResponseItemUserMessage(t *testing.T) {
	line := `{"type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"これはCodexの十分に長いテストプロンプトです"}]}}`
	path := writeJSONLFile(t, "session.jsonl", []string{line})

	prompts, err := history.ReadCodexPromptsFromFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(prompts) != 1 {
		t.Fatalf("expected 1 prompt, got %d", len(prompts))
	}
	if !strings.Contains(prompts[0].FullText, "Codex") {
		t.Errorf("expected Codex prompt text, got %q", prompts[0].FullText)
	}
}

func TestReadCodexPromptsFromFile_ParsesEventUserMessage(t *testing.T) {
	line := `{"type":"event_msg","payload":{"item":{"type":"UserMessage","content":[{"type":"text","text":"これはCodex eventの十分に長いプロンプトです"}]}}}`
	path := writeJSONLFile(t, "session.jsonl", []string{line})

	prompts, err := history.ReadCodexPromptsFromFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(prompts) != 1 {
		t.Fatalf("expected 1 prompt, got %d", len(prompts))
	}
	if !strings.Contains(prompts[0].FullText, "event") {
		t.Errorf("expected event prompt text, got %q", prompts[0].FullText)
	}
}

func TestReadRandomPrompts_ReadsCodexSessionsWhenClaudeHistoryMissing(t *testing.T) {
	home := t.TempDir()
	sessionDir := filepath.Join(home, ".codex", "sessions", "2026", "09", "11")
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatalf("mkdir codex sessions: %v", err)
	}
	line := `{"type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"これはCodex sessionsだけにある長いプロンプトです"}]}}`
	if err := os.WriteFile(filepath.Join(sessionDir, "session.jsonl"), []byte(line), 0o644); err != nil {
		t.Fatalf("write codex session: %v", err)
	}

	t.Setenv("HOME", home)
	prompts, err := history.ReadRandomPrompts(10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(prompts) != 1 {
		t.Fatalf("expected 1 prompt, got %d", len(prompts))
	}
	if !strings.Contains(prompts[0].FullText, "Codex sessions") {
		t.Errorf("expected Codex sessions prompt, got %q", prompts[0].FullText)
	}
}
