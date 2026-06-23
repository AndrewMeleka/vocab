package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/andrewnageh/vocab/internal/config"
)

type Client struct {
	host  string
	model string
	http  *http.Client
}

func New(cfg config.Config) *Client {
	return &Client{
		host:  strings.TrimRight(cfg.OllamaHost, "/"),
		model: cfg.Model,
		http:  &http.Client{Timeout: 60 * time.Second},
	}
}

func (c *Client) Host() string  { return c.host }
func (c *Client) Model() string { return c.model }

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatReq struct {
	Model    string         `json:"model"`
	Messages []chatMessage  `json:"messages"`
	Stream   bool           `json:"stream"`
	Format   string         `json:"format,omitempty"`
	Think    bool           `json:"think"`
	Opts     map[string]any `json:"options,omitempty"`
}

type chatResp struct {
	Message    chatMessage `json:"message"`
	Error      string      `json:"error,omitempty"`
	DoneReason string      `json:"done_reason,omitempty"`
}

func (c *Client) generate(ctx context.Context, prompt string, jsonFormat bool) (string, error) {
	body := chatReq{
		Model:    c.model,
		Messages: []chatMessage{{Role: "user", Content: prompt}},
		Stream:   false,
		Think:    false, // disable Qwen3-style thinking; required for JSON-format compat
		Opts: map[string]any{
			"num_predict":    1024,
			"temperature":    0.3,
			"top_p":          0.9,
			"top_k":          40,
			"repeat_penalty": 1.4,
			"repeat_last_n":  256,
		},
	}
	if jsonFormat {
		body.Format = "json"
	}
	buf, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.host+"/api/chat", bytes.NewReader(buf))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("ollama unreachable at %s: %w", c.host, err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ollama %s: %s", resp.Status, string(data))
	}
	var out chatResp
	if err := json.Unmarshal(data, &out); err != nil {
		return "", fmt.Errorf("decode ollama response: %w", err)
	}
	if out.Error != "" {
		return "", fmt.Errorf("ollama: %s", out.Error)
	}
	content := strings.TrimSpace(out.Message.Content)
	if content == "" {
		hint := ""
		if out.DoneReason != "" {
			hint = fmt.Sprintf(" (done_reason: %s)", out.DoneReason)
		}
		return "", fmt.Errorf("model %q on %s returned empty response%s — verify the model is installed (`ollama list` on the host) and supports JSON output",
			c.model, c.host, hint)
	}
	return content, nil
}

// Ping does a tiny generate call to verify the host+model are reachable.
func (c *Client) Ping(ctx context.Context) error {
	_, err := c.generate(ctx, "Respond with the single word: ok", false)
	return err
}

type Definition struct {
	Word       string   `json:"word"`
	Definition string   `json:"definition"`
	Examples   []string `json:"examples"`
}

func (c *Client) Define(ctx context.Context, word string) (Definition, error) {
	prompt := fmt.Sprintf(`You are an English dictionary. Define the word %q.
Respond ONLY with JSON of shape:
{"word": "...", "definition": "one clear sentence", "examples": ["sentence 1", "sentence 2", "sentence 3"]}
Examples should be natural usage in distinct contexts.`, word)
	raw, err := c.generate(ctx, prompt, true)
	if err != nil {
		return Definition{}, err
	}
	var d Definition
	if err := json.Unmarshal([]byte(raw), &d); err != nil {
		return Definition{}, fmt.Errorf("parse definition JSON: %w (raw: %s)", err, raw)
	}
	if d.Word == "" {
		d.Word = word
	}
	return d, nil
}

func (c *Client) MoreExamples(ctx context.Context, word, definition string, existing []string) ([]string, error) {
	prompt := fmt.Sprintf(`Provide 3 NEW example sentences using the English word %q.
Definition: %s
Avoid these existing examples: %s
Respond ONLY with JSON: {"examples": ["...", "...", "..."]}`,
		word, definition, strings.Join(existing, " | "))
	raw, err := c.generate(ctx, prompt, true)
	if err != nil {
		return nil, err
	}
	var out struct {
		Examples []string `json:"examples"`
	}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, fmt.Errorf("parse examples JSON: %w (raw: %s)", err, raw)
	}
	return out.Examples, nil
}

type WordOfDay struct {
	Word   string `json:"word"`
	Reason string `json:"reason"`
}

func (c *Client) PickWordOfDay(ctx context.Context, candidates []string) (WordOfDay, error) {
	if len(candidates) == 0 {
		return WordOfDay{}, fmt.Errorf("no candidate words")
	}
	prompt := fmt.Sprintf(`From this list of English words, pick ONE as "word of the day".
Choose the most interesting based on etymological richness, vivid imagery, or unusual meaning.
Words: %s
Respond ONLY with JSON: {"word": "exact word from list", "reason": "one short sentence"}`,
		strings.Join(candidates, ", "))
	raw, err := c.generate(ctx, prompt, true)
	if err != nil {
		return WordOfDay{}, err
	}
	var w WordOfDay
	if err := json.Unmarshal([]byte(raw), &w); err != nil {
		return WordOfDay{}, fmt.Errorf("parse word-of-day JSON: %w (raw: %s)", err, raw)
	}
	return w, nil
}

func (c *Client) MicroStory(ctx context.Context, words []string) (string, error) {
	prompt := fmt.Sprintf(`Write a vivid micro-story (under 120 words) that naturally uses ALL of these English words at least once: %s.
Bold each target word using **double asterisks**. Return only the story, no preamble.`,
		strings.Join(words, ", "))
	out, err := c.generate(ctx, prompt, false)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

type Suggestion struct {
	Word string `json:"word"`
	Hint string `json:"hint"`
}

type WordValidation struct {
	Valid bool   `json:"valid"`
	Type  string `json:"type"`
}

// ValidateEnglishWord checks whether word is a real English dictionary word and
// returns its part-of-speech. Used as the AI gate in `add` for words missing
// from the local WordNet seed.
func (c *Client) ValidateEnglishWord(ctx context.Context, word string) (WordValidation, error) {
	prompt := fmt.Sprintf(`Is %q a real word in the standard English dictionary?
Respond ONLY with JSON: {"valid": true|false, "type": "noun|verb|adj|adv|pron|prep|conj|interj|phrase"}.
If "valid" is false, set "type" to an empty string.
Do NOT accept proper nouns, brand names, slang neologisms, or typos.`, word)
	raw, err := c.generate(ctx, prompt, true)
	if err != nil {
		return WordValidation{}, err
	}
	var v WordValidation
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		return WordValidation{}, fmt.Errorf("parse validation JSON: %w (raw: %s)", err, raw)
	}
	return v, nil
}

func (c *Client) SuggestDaily(ctx context.Context, knownWords []string, n int, levels []string) ([]Suggestion, error) {
	levelClause := ""
	if len(levels) > 0 {
		levelClause = fmt.Sprintf(" Target CEFR level(s): %s.", strings.Join(levels, ", "))
	}
	prompt := fmt.Sprintf(`Suggest %d distinct English vocabulary words a curious learner should study today.%s
Each entry's "word" MUST be a single English word — no spaces, no punctuation, no digits, no CEFR codes (a1/b2/etc).
Each word MUST be unique within the response.
They should NOT appear in this known list: %s
Prefer rich, expressive, mid-frequency words (not too obscure, not trivial).
Respond ONLY with JSON: {"words": [{"word": "<single english word>", "hint": "very short reason"}]}`,
		n, levelClause, strings.Join(knownWords, ", "))
	raw, err := c.generate(ctx, prompt, true)
	if err != nil {
		return nil, err
	}
	var parsed []Suggestion
	var out struct {
		Words []Suggestion `json:"words"`
	}
	if jerr := json.Unmarshal([]byte(raw), &out); jerr == nil {
		parsed = out.Words
	} else {
		parsed = salvageSuggestions(raw)
		if len(parsed) == 0 {
			return nil, fmt.Errorf("parse suggestions JSON: %w (raw: %s)", jerr, raw)
		}
	}
	return cleanSuggestions(parsed), nil
}

func cleanSuggestions(in []Suggestion) []Suggestion {
	seen := map[string]bool{}
	var clean []Suggestion
	for _, s := range in {
		w := strings.ToLower(strings.TrimSpace(s.Word))
		if w == "" || !isSingleWord(w) || seen[w] {
			continue
		}
		seen[w] = true
		s.Word = w
		clean = append(clean, s)
	}
	return clean
}

var suggestionObjectRe = regexp.MustCompile(`\{\s*"word"\s*:\s*"([^"]+)"\s*,\s*"hint"\s*:\s*"([^"]*)"\s*\}`)

// salvageSuggestions extracts complete {"word","hint"} objects from a possibly
// truncated raw response — useful when a token loop ate the closing braces.
func salvageSuggestions(raw string) []Suggestion {
	matches := suggestionObjectRe.FindAllStringSubmatch(raw, -1)
	out := make([]Suggestion, 0, len(matches))
	for _, m := range matches {
		out = append(out, Suggestion{Word: m[1], Hint: m[2]})
	}
	return out
}

func isSingleWord(s string) bool {
	if len(s) == 0 || len(s) > 40 {
		return false
	}
	for _, r := range s {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '-' || r == '\'') {
			return false
		}
	}
	return true
}
