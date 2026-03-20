# nmail

한국 이메일 서비스(네이버, 다음)를 위한 CLI. OpenClaw 에이전트가 쓰기 편하게 설계됨.

> Korean email CLI for agents and humans. JSON output by default.

## Features

- 🇰🇷 **Korean email first** — Naver, Daum presets, EUC-KR auto-decode
- 🤖 **Agent-first** — JSON output by default, `--pretty` for humans
- 📦 **Single binary** — Go, zero dependencies at runtime
- 🔌 **OpenClaw skill** — Install via ClewHub

## Installation

```bash
# Coming soon: brew install harlock/tap/nmail
```

## Setup

```bash
# Add Naver account (uses app password)
nmail config add --provider naver --email you@naver.com --password <app-password>
```

> **App password:** Naver 계정 → 보안설정 → 2단계 인증 → 애플리케이션 비밀번호 생성

## Usage

```bash
# List inbox (JSON, default)
nmail inbox --limit 20

# Human-readable
nmail inbox --pretty

# Read a message
nmail read 42

# Send
nmail send --to friend@naver.com --subject "안녕" --body "잘 지내?"

# Send from file
nmail send --to friend@naver.com --subject "긴 메일" --body-file ./message.txt
```

## Status

| Phase | Status |
|-------|--------|
| Phase 1: Scaffolding | ✅ Done |
| Phase 2: Config | ✅ Done |
| Phase 3: Inbox & Read | ✅ Done |
| Phase 4: Send | ✅ Done |
| Phase 5: ClewHub Skill | ✅ SKILL.md 작성 완료 — 연동 테스트 후 등록 |

## License

MIT © 2026 Harlock Choi
