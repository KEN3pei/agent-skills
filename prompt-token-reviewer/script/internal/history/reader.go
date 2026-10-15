package history

import (
	"bufio"
	"encoding/json"
	"errors"
	"math/rand"
	"os"
	"path/filepath"
)

// minDisplayLength はフィルタ条件：display のルーン数がこれ以上なら有効とみなす。
// これによって/exitのようなコマンド系や簡単すぎるプロンプトはレビュー対象から外している。
const minDisplayLength = 15

// Prompt は会話履歴から抽出した1件のユーザープロンプトを表す。
type Prompt struct {
	Display  string
	FullText string
}

// historyEntry は history.jsonl の1行を表す。
type historyEntry struct {
	Display        string                   `json:"display"`
	PastedContents map[string]pastedContent `json:"pastedContents"`
}

type pastedContent struct {
	ID      int    `json:"id"`
	Type    string `json:"type"`
	Content string `json:"content"`
}

type codexEntry struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

type codexResponsePayload struct {
	Type    string         `json:"type"`
	Role    string         `json:"role"`
	Content []codexContent `json:"content"`
}

type codexEventPayload struct {
	Item codexItem `json:"item"`
}

type codexItem struct {
	Type    string         `json:"type"`
	Content []codexContent `json:"content"`
}

type codexContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// ReadRandomPrompts は Claude Code と Codex のローカル履歴からランダムに最大 limit 件のプロンプトを返す。
func ReadRandomPrompts(limit int) ([]Prompt, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	var prompts []Prompt
	claudePath := filepath.Join(home, ".claude", "history.jsonl")
	if claudePrompts, err := ReadPromptsFromFile(claudePath, 0); err == nil {
		prompts = append(prompts, claudePrompts...)
	}

	codexRoot := filepath.Join(home, ".codex", "sessions")
	if codexPrompts, err := ReadCodexPromptsFromDir(codexRoot); err == nil {
		prompts = append(prompts, codexPrompts...)
	}

	return samplePrompts(dedupePrompts(prompts), limit)
}

// ReadPromptsFromFile は指定パスの JSONL ファイルからランダムに最大 limit 件のプロンプトを返す。
// テスト可能にするため path を引数に取る。
func ReadPromptsFromFile(path string, limit int) ([]Prompt, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var filtered []Prompt
	scanner := bufio.NewScanner(f)
	// 長い行にも対応できるよう 1MB バッファを設定する
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		var entry historyEntry
		if err := json.Unmarshal(line, &entry); err != nil {
			// 不正な JSON 行はスキップする
			continue
		}

		pastedText := collectPastedText(entry.PastedContents)
		hasMeaningfulPasted := pastedText != ""
		hasLongDisplay := len([]rune(entry.Display)) >= minDisplayLength

		if !hasLongDisplay && !hasMeaningfulPasted {
			continue
		}

		fullText := entry.Display
		if pastedText != "" {
			fullText += "\n" + pastedText
		}

		filtered = append(filtered, Prompt{
			Display:  entry.Display,
			FullText: fullText,
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return samplePrompts(filtered, limit)
}

// ReadCodexPromptsFromDir は ~/.codex/sessions 配下の JSONL ファイルからユーザープロンプトを返す。
func ReadCodexPromptsFromDir(root string) ([]Prompt, error) {
	var prompts []Prompt
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() || filepath.Ext(path) != ".jsonl" {
			return nil
		}
		filePrompts, err := ReadCodexPromptsFromFile(path)
		if err != nil {
			return nil
		}
		prompts = append(prompts, filePrompts...)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(prompts) == 0 {
		return nil, errors.New("Codex sessions に有効なプロンプトが見つかりませんでした")
	}
	return dedupePrompts(prompts), nil
}

// ReadCodexPromptsFromFile は Codex session JSONL からユーザープロンプトを返す。
func ReadCodexPromptsFromFile(path string) ([]Prompt, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var prompts []Prompt
	scanner := bufio.NewScanner(f)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		var entry codexEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			continue
		}

		for _, text := range codexPromptTexts(entry) {
			if len([]rune(text)) < minDisplayLength {
				continue
			}
			prompts = append(prompts, Prompt{Display: text, FullText: text})
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(prompts) == 0 {
		return nil, errors.New("有効な Codex プロンプトが見つかりませんでした")
	}
	return dedupePrompts(prompts), nil
}

func samplePrompts(prompts []Prompt, limit int) ([]Prompt, error) {
	if len(prompts) == 0 {
		return nil, errors.New("有効なプロンプトが見つかりませんでした（全エントリが短すぎるか空です）")
	}
	rand.Shuffle(len(prompts), func(i, j int) {
		prompts[i], prompts[j] = prompts[j], prompts[i]
	})

	n := limit
	if n <= 0 || len(prompts) < n {
		n = len(prompts)
	}
	return prompts[:n], nil
}

func dedupePrompts(prompts []Prompt) []Prompt {
	seen := make(map[string]bool, len(prompts))
	var unique []Prompt
	for _, prompt := range prompts {
		if prompt.FullText == "" || seen[prompt.FullText] {
			continue
		}
		seen[prompt.FullText] = true
		unique = append(unique, prompt)
	}
	return unique
}

func codexPromptTexts(entry codexEntry) []string {
	switch entry.Type {
	case "response_item":
		var payload codexResponsePayload
		if err := json.Unmarshal(entry.Payload, &payload); err != nil || payload.Type != "message" || payload.Role != "user" {
			return nil
		}
		return collectCodexText(payload.Content)
	case "event_msg":
		var payload codexEventPayload
		if err := json.Unmarshal(entry.Payload, &payload); err != nil || payload.Item.Type != "UserMessage" {
			return nil
		}
		return collectCodexText(payload.Item.Content)
	default:
		return nil
	}
}

func collectCodexText(contents []codexContent) []string {
	var texts []string
	for _, content := range contents {
		if (content.Type == "input_text" || content.Type == "text") && content.Text != "" {
			texts = append(texts, content.Text)
		}
	}
	return texts
}

// collectPastedText は pastedContents から type=="text" のコンテンツを結合して返す。
// history.jsonlの履歴からtype=imageのものはレビューできないのでここで対象から外している。
func collectPastedText(contents map[string]pastedContent) string {
	var result string
	for _, c := range contents {
		if c.Type == "text" && c.Content != "" {
			if result != "" {
				result += "\n"
			}
			result += c.Content
		}
	}
	return result
}
