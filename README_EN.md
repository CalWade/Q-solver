<div align="center">
  <img src="assets/banner.jpg" alt="Q-Solver Banner" width="100%">

  <h1>Q-Solver</h1>
  <p><strong>An AI desktop problem-solving assistant.</strong></p>
  <p>Capture a question, keep the window floating, and get answers fast. Supports hidden mode, click-through, and low-distraction use with your own API key.</p>

  <p>
    <a href="https://github.com/jym66/Q-solver/stargazers"><img src="https://img.shields.io/github/stars/jym66/Q-solver?color=ffcb6b&style=for-the-badge&labelColor=30363d" alt="Stars"></a>
    <a href="https://github.com/jym66/Q-solver/releases"><img src="https://img.shields.io/github/v/release/jym66/Q-solver?color=89d185&style=for-the-badge&labelColor=30363d" alt="Release"></a>
    <img src="https://img.shields.io/badge/Go-1.25+-00ADD8?style=for-the-badge&logo=go&logoColor=white&labelColor=30363d" alt="Go">
    <img src="https://img.shields.io/badge/Vue-3.x-4FC08D?style=for-the-badge&logo=vue.js&logoColor=white&labelColor=30363d" alt="Vue">
    <img src="https://img.shields.io/badge/Wails-v2-E30613?style=for-the-badge&logo=wails&logoColor=white&labelColor=30363d" alt="Wails">
  </p>

  <p>
    <img src="https://img.shields.io/badge/macOS-000000?style=flat-square&logo=apple&logoColor=white" alt="macOS">
    <img src="https://img.shields.io/badge/Windows-0078D6?style=flat-square&logo=windows&logoColor=white" alt="Windows">
  </p>

  <p>
    <a href="README.md">中文</a>
  </p>
</div>

---

## What This Is

Q-Solver is an AI desktop problem-solving assistant.

When you see a question, a code error, a formula, a chart, or English content on screen, you do not need to switch tabs and copy-paste everything into a chatbot. Just take a screenshot and send it straight to your model for analysis.

Its core strengths are:

- screenshot-first solving for coding questions, tests, interviews, and debugging
- hidden mode for lower-profile use
- floating output that does not keep stealing focus
- click-through support so it can stay at the edge of your screen
- system capture protection settings, with sharing results verified separately for each OS, platform and capture mode

---

## Q-Solver in 5 Screens

### 1. Welcome and Runtime Status: know your setup immediately

<img src="assets/img1.png" alt="Q-Solver Welcome and Runtime Status" width="100%">

- the welcome screen shows the core shortcuts right away
- the status panel shows your API key status, base URL, active model, and hidden mode
- a good fit for an always-available desktop solving assistant

### 2. Screenshot Settings: decide whether you want more clarity or more speed

<img src="assets/img2.png" alt="Q-Solver Screenshot Settings" width="100%">

- supports region capture and fullscreen capture
- lets you choose original upload or compressed upload
- adjustable compression, sharpening, and grayscale settings

### 3. Model and Scene Presets: same model, different solving styles

<img src="assets/img3.png" alt="Q-Solver Model and Scene Presets" width="100%">

- choose the model you want to use
- switch between different role or scene prompts
- make the same screenshot produce output that better matches the current task

### 4. API Setup: choose a provider and enter your own key

<img src="assets/img4.png" alt="Q-Solver API Setup" width="100%">

- supports OpenAI-compatible APIs
- includes presets for OpenAI, Google, Anthropic, DeepSeek, Alibaba Cloud, Moonshot, OpenRouter, and more
- when you choose `Custom`, you can enter your own `Base URL`

### 5. Result View: built for continuous reading and follow-up questions

<img src="assets/img5.png" alt="Q-Solver Result View" width="100%">

- the history panel on the left makes it easy to revisit previous questions
- the answer area is structured analysis, not just a one-line reply
- better suited for reading reasoning, formulas, code, and key steps while you work

---

## Core Features

- AI problem-solving assistant: capture content from your screen and send it directly to your model
- History panel: keep recent results for continuous review
- Model switching: automatically load available models from your current endpoint
- Scene presets: switch prompt styles for different question types
- Custom API support: works with any OpenAI-compatible service
- Screenshot tuning: compression, sharpening, grayscale, and original upload options
- Resume parsing: import a PDF resume and turn it into Markdown with your own model
- Desktop floating window: hidden mode, click-through, and shortcut-based control
- Low-distraction workflow: designed to stay available without constantly stealing focus

---

## Quick Start

### Option 1: Download the App

Download the latest release for your system from [Releases](https://github.com/jym66/Q-solver/releases).

> [!NOTE]
> If macOS blocks the app on first launch, run:
> ```bash
> xattr -cr /Applications/Q-Solver.app
> chmod +x /Applications/Q-Solver.app/Contents/MacOS/Q-Solver
> ```

### Option 2: Run from Source

Requirements:

- Go 1.25+
- Node.js 22+
- Git and native build tools; project scripts prepare the pinned Wails version

```bash
git clone https://github.com/CalWade/Q-solver.git
cd Q-Solver
npm --prefix frontend ci
node scripts/run.mjs dev
```

Build a production package:

```bash
node scripts/run.mjs build
```

`dev` starts the isolated Vue UI and Go test services with synthetic data. `build` packages the native application without launching it. Run `node scripts/run.mjs check` for automated tests. See [development](docs/development.md), [architecture](docs/architecture.md) and [compatibility evidence](docs/compatibility/README.md). Scripts verify and prepare the pinned macOS panel patch; no global Wails CLI is required.

---

## Configuration

Recommended first-run flow:

1. Open Settings
2. In the `API` tab, choose a provider and enter your `API Key`
3. If you use a custom compatible endpoint, also enter `Base URL`
4. In the `Model` tab, refresh the model list and choose one
5. In the `Model` tab, choose the scene you want to use
6. In the `Screenshot` tab, adjust screenshot settings if needed
7. Go back to the main window and start solving

---

## Shortcuts

Default shortcuts:

| Action | Windows | macOS |
|:---|:---:|:---:|
| Screenshot | `F8` | `Cmd + 1` |
| Send to solve | `Ctrl + J` | `Cmd + J` |
| Show / Hide window | `F9` | `Cmd + 2` |
| Click-through | `F10` | `Cmd + 3` |
| Move window slightly | `Alt + Arrow Keys` | `Cmd + Option + Arrow Keys` |
| Scroll content | `Alt + PgUp / PgDn` | `Cmd + Option + Shift + Arrow Keys` |

> Both platforms support recording and adjusting shortcuts. Keyboard protection is enabled separately; check the reported runtime status.

---

## Tech Stack

- Core: Go
- Desktop Binding: Wails
- Frontend: Vue 3 + Pinia
- LLM Access: OpenAI-compatible API + OpenAI SDK

---

## Star History

<div align="center">
  <a href="https://star-history.com/#jym66/Q-solver&Date">
    <img alt="Star History Chart" src="https://api.star-history.com/svg?repos=jym66/Q-solver&type=Date" />
  </a>
</div>

---

## License

This project is published under **CC BY-NC 4.0** as source-available software for personal, non-commercial use.

---

<div align="center">
  <p>If you like this screenshot-first AI workflow, a Star would mean a lot.</p>
  <p><a href="https://github.com/jym66">jym66</a></p>
</div>
