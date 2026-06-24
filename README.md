# 🧠 vocab

> An AI-powered, spaced-repetition vocabulary trainer that lives in your terminal.

`vocab` helps you grow your English vocabulary using a **Leitner spaced-repetition system** and a local **[Ollama](https://ollama.com) LLM** that doubles as your dictionary and tutor — generating definitions, example sentences, a word of the day, and even micro-stories woven from the words you're learning.

Everything runs **locally**: your collection lives in a SQLite database and the model runs on your own machine. No accounts, no cloud, no API keys.

---

## ✨ Features

- 📚 **Offline dictionary** — ships with a WordNet-seeded SQLite dictionary, so common words resolve instantly without touching the model.
- 🤖 **AI fallback** — unknown words are validated, defined, and given example sentences by your local Ollama model.
- 🗂️ **Leitner spaced repetition** — cards move through 5 boxes with configurable review intervals (`1, 3, 7, 14, 30` days by default).
- 🖥️ **Interactive dashboard** — a clean terminal UI (built with [Bubble Tea](https://github.com/charmbracelet/bubbletea)) showing your collection at a glance.
- 📝 **Review mode** — flip cards, mark them *knew* / *forgot*, pull more examples on demand, or test your spelling.
- 🌅 **Word of the day** + **daily suggestions** — fresh words sampled from the dictionary, filtered by CEFR level (A1–C2).
- 📖 **Micro-stories** — the model weaves your due/recent words into a short story to reinforce them in context.
- 🔍 **Multi-word search** — look up several words in one command: `vocab search hello world`.
- 🎚️ **Configurable** — model, host, daily/story word counts, box intervals, and learning level all live in a simple TOML file.

---

## 🚀 Installation

**Prerequisites**

- [Ollama](https://ollama.com) running locally with a model pulled:

  ```sh
  ollama pull llama3.2
  ```

- _(Optional)_ A system **text-to-speech** binary, used only by the **spelling
  test** in review mode to read words aloud. `vocab` uses the first one it finds:

  | Platform | Binary | How to get it |
  | --- | --- | --- |
  | macOS | `say` | Bundled with macOS — nothing to install. |
  | Linux | `espeak` | `sudo apt install espeak` (or your distro's equivalent). |
  | Linux | `spd-say` | Ships with `speech-dispatcher` (`sudo apt install speech-dispatcher`). |

  If none are available, every other feature still works — only the spelling
  test's audio is skipped.

### Quick install (recommended)

Grab a prebuilt binary for your platform with a single command:

```sh
curl -fsSL https://raw.githubusercontent.com/AndrewMeleka/vocab/main/install.sh | sh
```

This downloads the latest release, verifies its checksum, and installs `vocab`
to `/usr/local/bin` (falling back to `~/.local/bin` if that isn't writable).

Install a specific version or choose the location:

```sh
curl -fsSL https://raw.githubusercontent.com/AndrewMeleka/vocab/main/install.sh | VERSION=v0.1.0 sh
curl -fsSL https://raw.githubusercontent.com/AndrewMeleka/vocab/main/install.sh | BIN_DIR=$HOME/bin sh
```

Prefer not to pipe to a shell? Download the archive for your platform from the
[releases page](https://github.com/AndrewMeleka/vocab/releases), extract it, and
move the `vocab` binary onto your `PATH`.

### Build from source

Requires [Go](https://go.dev/dl/) 1.26+.

```sh
git clone https://github.com/AndrewMeleka/vocab.git
cd vocab
go build -o bin/vocab .
```

The binary is now at `./bin/vocab`. (Move it onto your `PATH` to run it as just `vocab`.)

---

## ⚡ Quick start

```sh
# Add words to your collection (offline lookup, AI fallback)
vocab add serendipity

# Look up one or more words
vocab search hello world ephemeral

# Get your word of the day + suggestions, and accept them
vocab daily --accept

# Review everything due today
vocab review

# Open the interactive dashboard
vocab
```

---

## 📋 Commands

| Command | What it does |
| --- | --- |
| `vocab` | Open the interactive dashboard (default when run with no args). |
| `vocab add <word>` | Add a word to your collection — local dictionary first, AI fallback if unknown. |
| `vocab search <word> [word...]` | Look up one or more words; AI-adds them to the dictionary + collection if missing. |
| `vocab review` | Review all cards due today in the interactive review UI. |
| `vocab daily` | Show the word of the day and suggest new words to learn. |
| `vocab story` | Generate a micro-story using your recent / due words. |
| `vocab list` | List your cards, with due status and Leitner box. |
| `vocab remove <word>` | Remove a card from your collection (dictionary entry is kept). Alias: `rm`. |
| `vocab reset` | Delete every card from your collection (asks for confirmation). |
| `vocab config` | Show config, DB stats, and verify the Ollama connection. |

### Useful flags

```sh
vocab list --box 2                 # filter to a specific Leitner box (0–4)
vocab daily --accept               # add all suggestions to your collection
vocab daily --level b2,c1          # set CEFR level(s) (persisted to config)
vocab reset --yes                  # skip the confirmation prompt
```

---

## ⌨️ Keybindings

**Dashboard**

| Key | Action |
| --- | --- |
| `r` | Start reviewing due cards |
| `q` / `esc` / `ctrl+c` | Quit |

**Review**

| Key | Action |
| --- | --- |
| `space` / `enter` | Reveal the card |
| `j` | Mark as *knew* (card promotes to the next box) |
| `f` | Mark as *forgot* (card resets) |
| `e` | Generate more examples |
| `s` | Spell-test mode |
| `q` / `esc` | Quit |

---

## ⚙️ Configuration

On first run, `vocab` writes a config file and a SQLite database to `~/.config/vocab/`:

- `~/.config/vocab/config.toml` — settings
- `~/.config/vocab/vocab.db` — dictionary + your collection

Run `vocab config` to see the resolved paths and stats.

```toml
# ~/.config/vocab/config.toml
ollama_host      = "http://localhost:11434"
model            = "llama3.2"
daily_word_count = 3
story_word_count = 5
box_interval_days = [1, 3, 7, 14, 30]   # Leitner intervals per box
level            = ["b1", "b2"]          # CEFR levels for daily suggestions
```

| Setting | Description |
| --- | --- |
| `ollama_host` | URL of your Ollama server. |
| `model` | Ollama model used for definitions, examples, and stories. |
| `daily_word_count` | How many new words `daily` suggests. |
| `story_word_count` | How many words `story` weaves together. |
| `box_interval_days` | Days until next review for each of the 5 Leitner boxes. |
| `level` | CEFR level(s) for daily suggestions — any of `a1, a2, b1, b2, c1, c2`. |

---

## 🧩 How it works

1. **Lookups hit the local dictionary first.** A WordNet-seeded SQLite store resolves most words instantly and offline.
2. **Unknown words fall back to the model.** Ollama validates the word is real English, then generates a definition and example sentences, which are saved back to the dictionary.
3. **Spaced repetition schedules your reviews.** Each card lives in a Leitner box; getting it right promotes it (longer interval), getting it wrong resets it (review again soon).
4. **The model reinforces in context** via the word of the day and micro-stories built from words you're actively learning.

---

## 🛠️ Tech stack

- **[Go](https://go.dev)** + **[Cobra](https://github.com/spf13/cobra)** for the CLI
- **[Bubble Tea](https://github.com/charmbracelet/bubbletea)** + **[Lip Gloss](https://github.com/charmbracelet/lipgloss)** for the terminal UI
- **[modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite)** — pure-Go SQLite (no CGo)
- **[Ollama](https://ollama.com)** for local LLM inference
- **WordNet 3.1** as the seed dictionary

---

## 📄 License

See [LICENSE](LICENSE) if present, otherwise all rights reserved by the author.
