package main

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

type app struct {
	home string
}

type thread struct {
	ID         string
	Rollout    string
	Title      string
	UpdatedAt  int64
	UpdatedMS  sql.NullInt64
	RecencyAt  int64
	RecencyMS  int64
	ColumnData map[string]any
}

type turn struct {
	TurnID            string
	RolloutOrdinal    int
	RolloutEndOrdinal sql.NullInt64
	Status            string
	CompletedAt       sql.NullInt64
}

type lineOffset struct {
	Start int64
	End   int64
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	home := defaultCodexHome()
	fs := flag.NewFlagSet("codex-rewind", flag.ExitOnError)
	fs.StringVar(&home, "codex-home", home, "Codex home directory")
	if err := fs.Parse(args); err != nil {
		return err
	}
	rest := fs.Args()
	if len(rest) == 0 {
		return errors.New("usage: codex-rewind [--codex-home PATH] <list-turns|clone-before-turn|clone-through-turn> ...")
	}

	a := app{home: home}
	switch rest[0] {
	case "list-turns":
		sub := flag.NewFlagSet("list-turns", flag.ExitOnError)
		sessionID := sub.String("session-id", "", "Source Codex session id")
		if err := sub.Parse(rest[1:]); err != nil {
			return err
		}
		if *sessionID == "" {
			return errors.New("--session-id is required")
		}
		return a.listTurns(*sessionID)
	case "clone-before-turn":
		sub := flag.NewFlagSet("clone-before-turn", flag.ExitOnError)
		sessionID := sub.String("session-id", "", "Source Codex session id")
		beforeTurn := sub.String("before-turn", "", "First turn to exclude")
		if err := sub.Parse(rest[1:]); err != nil {
			return err
		}
		if *sessionID == "" || *beforeTurn == "" {
			return errors.New("--session-id and --before-turn are required")
		}
		cutoff, err := a.cutoffBeforeTurn(*sessionID, *beforeTurn)
		if err != nil {
			return err
		}
		return a.clone(*sessionID, cutoff)
	case "clone-through-turn":
		sub := flag.NewFlagSet("clone-through-turn", flag.ExitOnError)
		sessionID := sub.String("session-id", "", "Source Codex session id")
		throughTurn := sub.String("through-turn", "", "Last turn to keep")
		if err := sub.Parse(rest[1:]); err != nil {
			return err
		}
		if *sessionID == "" || *throughTurn == "" {
			return errors.New("--session-id and --through-turn are required")
		}
		cutoff, err := a.cutoffThroughTurn(*sessionID, *throughTurn)
		if err != nil {
			return err
		}
		return a.clone(*sessionID, cutoff)
	default:
		return fmt.Errorf("unknown command %q", rest[0])
	}
}

func defaultCodexHome() string {
	if v := os.Getenv("CODEX_HOME"); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".codex"
	}
	return filepath.Join(home, ".codex")
}

func (a app) stateDB() string   { return filepath.Join(a.home, "state_5.sqlite") }
func (a app) histDB() string    { return filepath.Join(a.home, "thread_history_1.sqlite") }
func (a app) indexPath() string { return filepath.Join(a.home, "session_index.jsonl") }

func openDB(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	return db, nil
}

func (a app) loadThread(sessionID string) (thread, error) {
	db, err := openDB(a.stateDB())
	if err != nil {
		return thread{}, err
	}
	defer db.Close()

	rows, err := db.Query("SELECT * FROM threads WHERE id = ?", sessionID)
	if err != nil {
		return thread{}, err
	}
	defer rows.Close()
	items, err := scanRows(rows)
	if err != nil {
		return thread{}, err
	}
	if len(items) == 0 {
		return thread{}, fmt.Errorf("session not found in state DB: %s", sessionID)
	}
	item := items[0]
	t := thread{
		ID:         stringValue(item["id"]),
		Rollout:    stringValue(item["rollout_path"]),
		Title:      stringValue(item["title"]),
		UpdatedAt:  int64Value(item["updated_at"]),
		RecencyAt:  int64Value(item["recency_at"]),
		RecencyMS:  int64Value(item["recency_at_ms"]),
		ColumnData: item,
	}
	return t, nil
}

func (a app) turns(sessionID string) ([]turn, error) {
	db, err := openDB(a.histDB())
	if err != nil {
		return nil, err
	}
	defer db.Close()
	rows, err := db.Query(`SELECT turn_id, rollout_ordinal, rollout_end_ordinal, status, completed_at
		FROM thread_turns WHERE thread_id = ? ORDER BY rollout_ordinal`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []turn
	for rows.Next() {
		var t turn
		if err := rows.Scan(&t.TurnID, &t.RolloutOrdinal, &t.RolloutEndOrdinal, &t.Status, &t.CompletedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (a app) listTurns(sessionID string) error {
	t, err := a.loadThread(sessionID)
	if err != nil {
		return err
	}
	turns, err := a.turns(sessionID)
	if err != nil {
		return err
	}
	msgs, err := messagesByTurn(t.Rollout)
	if err != nil {
		return err
	}
	fmt.Printf("session_id\t%s\n", sessionID)
	fmt.Printf("title\t%s\n", t.Title)
	fmt.Println("turn_id\tstatus\tordinals\tmessages")
	for _, turn := range turns {
		end := ""
		if turn.RolloutEndOrdinal.Valid {
			end = fmt.Sprintf("%d", turn.RolloutEndOrdinal.Int64)
		}
		fmt.Printf("%s\t%s\t%d..%s\t%s\n", turn.TurnID, turn.Status, turn.RolloutOrdinal, end, strings.Join(msgs[turn.TurnID], " | "))
	}
	return nil
}

func (a app) cutoffBeforeTurn(sessionID, turnID string) (int, error) {
	turns, err := a.turns(sessionID)
	if err != nil {
		return 0, err
	}
	for _, t := range turns {
		if t.TurnID == turnID {
			cutoff := t.RolloutOrdinal - 1
			if cutoff < 0 {
				return 0, errors.New("boundary would leave no valid rollout prefix")
			}
			return cutoff, nil
		}
	}
	return 0, fmt.Errorf("turn not found: %s", turnID)
}

func (a app) cutoffThroughTurn(sessionID, turnID string) (int, error) {
	turns, err := a.turns(sessionID)
	if err != nil {
		return 0, err
	}
	for _, t := range turns {
		if t.TurnID == turnID {
			if !t.RolloutEndOrdinal.Valid {
				return 0, fmt.Errorf("turn has no end ordinal: %s", turnID)
			}
			return int(t.RolloutEndOrdinal.Int64), nil
		}
	}
	return 0, fmt.Errorf("turn not found: %s", turnID)
}

func (a app) clone(sessionID string, cutoff int) error {
	srcThread, err := a.loadThread(sessionID)
	if err != nil {
		return err
	}
	if _, err := os.Stat(srcThread.Rollout); err != nil {
		return fmt.Errorf("source rollout: %w", err)
	}

	backupDir, err := a.backup(srcThread.Rollout)
	if err != nil {
		return err
	}

	newID := uuid.NewString()
	oldWindow, _ := firstWindowID(srcThread.Rollout)
	newWindow := uuid.NewString()
	dst := filepath.Join(filepath.Dir(srcThread.Rollout), fmt.Sprintf("rollout-%s-%s.jsonl", time.Now().Format("2006-01-02T15-04-05"), newID))
	if err := writeClonedRollout(srcThread.Rollout, dst, cutoff, sessionID, newID, oldWindow, newWindow); err != nil {
		return err
	}
	size, offsets, err := rolloutOffsets(dst)
	if err != nil {
		return err
	}
	if err := a.registerClone(srcThread, newID, dst, cutoff, size, offsets, sessionID, oldWindow, newWindow); err != nil {
		return err
	}

	result := map[string]any{
		"source_session_id": sessionID,
		"new_session_id":    newID,
		"cutoff_ordinal":    cutoff,
		"rollout_path":      dst,
		"backup_dir":        backupDir,
		"resume_command":    "codex resume " + newID,
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(result); err != nil {
		return err
	}
	fmt.Println()
	fmt.Println("Run this in another terminal:")
	fmt.Println("codex resume " + newID)
	return nil
}

func (a app) backup(sourceRollout string) (string, error) {
	dir := filepath.Join(a.home, "session-cleanup-backups", "codex-rewind-"+time.Now().Format("20060102-150405"))
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	for _, pair := range [][2]string{
		{sourceRollout, filepath.Join(dir, "source-rollout.jsonl")},
		{a.indexPath(), filepath.Join(dir, "session_index.jsonl")},
		{a.stateDB(), filepath.Join(dir, "state_5.sqlite")},
		{a.histDB(), filepath.Join(dir, "thread_history_1.sqlite")},
	} {
		if err := copyFile(pair[0], pair[1]); err != nil {
			return "", err
		}
	}
	return dir, nil
}

func (a app) registerClone(src thread, newID, rolloutPath string, cutoff int, rolloutSize int64, offsets map[int]lineOffset, oldID, oldWindow, newWindow string) error {
	state, err := openDB(a.stateDB())
	if err != nil {
		return err
	}
	defer state.Close()
	hist, err := openDB(a.histDB())
	if err != nil {
		return err
	}
	defer hist.Close()

	if _, err := state.Exec("BEGIN IMMEDIATE"); err != nil {
		return err
	}
	if _, err := hist.Exec("BEGIN IMMEDIATE"); err != nil {
		state.Exec("ROLLBACK")
		return err
	}
	rollback := func() {
		hist.Exec("ROLLBACK")
		state.Exec("ROLLBACK")
	}

	if _, err := state.Exec("DELETE FROM threads WHERE id = ?", newID); err != nil {
		rollback()
		return err
	}
	if _, err := hist.Exec("DELETE FROM thread_turns WHERE thread_id = ?", newID); err != nil {
		rollback()
		return err
	}
	if _, err := hist.Exec("DELETE FROM thread_items WHERE thread_id = ?", newID); err != nil {
		rollback()
		return err
	}
	if _, err := hist.Exec("DELETE FROM thread_history_projection_state WHERE thread_id = ?", newID); err != nil {
		rollback()
		return err
	}

	cleanUpdated, err := a.cleanUpdatedAt(hist, oldID, cutoff)
	if err != nil {
		rollback()
		return err
	}

	newThread := cloneMap(src.ColumnData)
	newThread["id"] = newID
	newThread["rollout_path"] = rolloutPath
	newThread["updated_at"] = cleanUpdated
	newThread["updated_at_ms"] = cleanUpdated * 1000
	newThread["recency_at"] = cleanUpdated
	newThread["recency_at_ms"] = cleanUpdated * 1000
	newThread["archived"] = int64(0)
	newThread["archived_at"] = nil
	newThread["tokens_used"] = int64(0)
	newThread["title"] = stringValue(newThread["title"]) + " (rewound)"
	if stringValue(newThread["name"]) != "" {
		newThread["name"] = stringValue(newThread["name"]) + " (rewound)"
	}
	if err := insertMap(state, "threads", newThread); err != nil {
		rollback()
		return err
	}

	if err := copyDynamicTools(state, oldID, newID); err != nil {
		rollback()
		return err
	}
	if err := copyTurns(hist, oldID, newID, cutoff, offsets); err != nil {
		rollback()
		return err
	}
	if err := copyItems(hist, oldID, newID, cutoff, oldWindow, newWindow); err != nil {
		rollback()
		return err
	}
	if _, err := hist.Exec(`INSERT INTO thread_history_projection_state
		(thread_id, next_rollout_byte_offset, next_rollout_ordinal) VALUES (?, ?, ?)`, newID, rolloutSize, cutoff+1); err != nil {
		rollback()
		return err
	}
	if err := appendSessionIndex(a.indexPath(), newID, stringValue(newThread["title"]), cleanUpdated); err != nil {
		rollback()
		return err
	}
	if _, err := hist.Exec("COMMIT"); err != nil {
		rollback()
		return err
	}
	if _, err := state.Exec("COMMIT"); err != nil {
		return err
	}
	return nil
}

func (a app) cleanUpdatedAt(hist *sql.DB, sessionID string, cutoff int) (int64, error) {
	var v sql.NullInt64
	err := hist.QueryRow(`SELECT completed_at FROM thread_turns
		WHERE thread_id = ? AND rollout_end_ordinal <= ?
		ORDER BY rollout_end_ordinal DESC LIMIT 1`, sessionID, cutoff).Scan(&v)
	if err != nil {
		return 0, err
	}
	if !v.Valid {
		return time.Now().Unix(), nil
	}
	return v.Int64, nil
}

func messagesByTurn(path string) (map[string][]string, error) {
	out := map[string][]string{}
	err := eachJSONLine(path, func(_ int64, _ int64, row map[string]any) error {
		payload := mapValue(row["payload"])
		if stringValue(payload["type"]) != "message" {
			return nil
		}
		meta := mapValue(payload["internal_chat_message_metadata_passthrough"])
		turnID := stringValue(meta["turn_id"])
		if turnID == "" {
			turnID = stringValue(payload["turn_id"])
		}
		role := stringValue(payload["role"])
		if turnID == "" || (role != "user" && role != "assistant") {
			return nil
		}
		content, _ := payload["content"].([]any)
		if len(content) == 0 {
			return nil
		}
		text := strings.Join(strings.Fields(stringValue(mapValue(content[0])["text"])), " ")
		if len(text) > 110 {
			text = text[:110]
		}
		out[turnID] = append(out[turnID], role+": "+text)
		return nil
	})
	return out, err
}

func firstWindowID(path string) (string, error) {
	found := ""
	err := eachJSONLine(path, func(_ int64, _ int64, row map[string]any) error {
		payload := mapValue(row["payload"])
		cw := mapValue(payload["context_window"])
		if id := stringValue(cw["window_id"]); id != "" {
			found = id
			return io.EOF
		}
		return nil
	})
	if errors.Is(err, io.EOF) {
		return found, nil
	}
	return found, err
}

func writeClonedRollout(src, dst string, cutoff int, oldID, newID, oldWindow, newWindow string) error {
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	return eachJSONLine(src, func(_ int64, _ int64, row map[string]any) error {
		if int64Value(row["ordinal"]) > int64(cutoff) {
			return io.EOF
		}
		row = replaceStrings(row, oldID, newID, oldWindow, newWindow).(map[string]any)
		b, err := json.Marshal(row)
		if err != nil {
			return err
		}
		_, err = out.Write(append(b, '\n'))
		return err
	})
}

func rolloutOffsets(path string) (int64, map[int]lineOffset, error) {
	offsets := map[int]lineOffset{}
	var total int64
	err := eachJSONLine(path, func(start int64, end int64, row map[string]any) error {
		offsets[int(int64Value(row["ordinal"]))] = lineOffset{Start: start, End: end}
		total = end
		return nil
	})
	return total, offsets, err
}

func eachJSONLine(path string, fn func(start int64, end int64, row map[string]any) error) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	reader := bufio.NewReader(f)
	var offset int64
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			start := offset
			offset += int64(len(line))
			var row map[string]any
			if err := json.Unmarshal(line, &row); err != nil {
				return err
			}
			if fnErr := fn(start, offset, row); fnErr != nil {
				if errors.Is(fnErr, io.EOF) {
					return nil
				}
				return fnErr
			}
		}
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

func scanRows(rows *sql.Rows) ([]map[string]any, error) {
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	var out []map[string]any
	for rows.Next() {
		values := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range values {
			ptrs[i] = &values[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		item := map[string]any{}
		for i, col := range cols {
			switch v := values[i].(type) {
			case []byte:
				item[col] = string(v)
			default:
				item[col] = v
			}
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func insertMap(db *sql.DB, table string, item map[string]any) error {
	cols := sortedKeys(item)
	args := make([]any, len(cols))
	holders := make([]string, len(cols))
	for i, col := range cols {
		args[i] = item[col]
		holders[i] = "?"
	}
	_, err := db.Exec(fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", table, strings.Join(cols, ","), strings.Join(holders, ",")), args...)
	return err
}

func copyDynamicTools(db *sql.DB, oldID, newID string) error {
	rows, err := db.Query("SELECT * FROM thread_dynamic_tools WHERE thread_id = ?", oldID)
	if err != nil {
		return err
	}
	defer rows.Close()
	items, err := scanRows(rows)
	if err != nil {
		return err
	}
	for _, item := range items {
		item["thread_id"] = newID
		if err := insertMap(db, "thread_dynamic_tools", item); err != nil {
			return err
		}
	}
	return nil
}

func copyTurns(db *sql.DB, oldID, newID string, cutoff int, offsets map[int]lineOffset) error {
	rows, err := db.Query("SELECT * FROM thread_turns WHERE thread_id = ? AND rollout_end_ordinal <= ? ORDER BY rollout_ordinal", oldID, cutoff)
	if err != nil {
		return err
	}
	defer rows.Close()
	items, err := scanRows(rows)
	if err != nil {
		return err
	}
	for _, item := range items {
		item["thread_id"] = newID
		start := offsets[int(int64Value(item["rollout_ordinal"]))]
		item["rollout_byte_offset"] = start.Start
		if item["rollout_end_ordinal"] != nil {
			end := offsets[int(int64Value(item["rollout_end_ordinal"]))]
			item["rollout_end_byte_offset"] = end.End
		}
		if err := insertMap(db, "thread_turns", item); err != nil {
			return err
		}
	}
	return nil
}

func copyItems(db *sql.DB, oldID, newID string, cutoff int, oldWindow, newWindow string) error {
	rows, err := db.Query("SELECT * FROM thread_items WHERE thread_id = ? AND rollout_ordinal <= ? ORDER BY rollout_ordinal", oldID, cutoff)
	if err != nil {
		return err
	}
	defer rows.Close()
	items, err := scanRows(rows)
	if err != nil {
		return err
	}
	for _, item := range items {
		item["thread_id"] = newID
		item["item_json"] = replaceString(stringValue(item["item_json"]), oldID, newID, oldWindow, newWindow)
		if err := insertMap(db, "thread_items", item); err != nil {
			return err
		}
	}
	return nil
}

func appendSessionIndex(path, id, title string, updatedAt int64) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	row := map[string]any{
		"id":          id,
		"thread_name": title,
		"updated_at":  time.Unix(updatedAt, 0).UTC().Format(time.RFC3339),
	}
	b, err := json.Marshal(row)
	if err != nil {
		return err
	}
	_, err = f.Write(append(b, '\n'))
	return err
}

func replaceStrings(v any, oldID, newID, oldWindow, newWindow string) any {
	switch x := v.(type) {
	case string:
		return replaceString(x, oldID, newID, oldWindow, newWindow)
	case []any:
		for i := range x {
			x[i] = replaceStrings(x[i], oldID, newID, oldWindow, newWindow)
		}
		return x
	case map[string]any:
		for k := range x {
			x[k] = replaceStrings(x[k], oldID, newID, oldWindow, newWindow)
		}
		return x
	default:
		return v
	}
}

func replaceString(s, oldID, newID, oldWindow, newWindow string) string {
	s = strings.ReplaceAll(s, oldID, newID)
	if oldWindow != "" {
		s = strings.ReplaceAll(s, oldWindow, newWindow)
	}
	return s
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

func cloneMap(src map[string]any) map[string]any {
	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func mapValue(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return map[string]any{}
}

func stringValue(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case []byte:
		return string(x)
	default:
		return ""
	}
}

func int64Value(v any) int64 {
	switch x := v.(type) {
	case int64:
		return x
	case int:
		return int64(x)
	case float64:
		return int64(x)
	case []byte:
		var n int64
		fmt.Sscan(string(x), &n)
		return n
	case string:
		var n int64
		fmt.Sscan(x, &n)
		return n
	default:
		return 0
	}
}
