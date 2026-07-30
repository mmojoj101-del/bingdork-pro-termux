# 📱 BingDork Pro Termux

**Advanced Search Automation Framework** with **Termux TUI** for Android

## ⚡ Quick Start (on your phone)

### 1. Install Termux
Get it from [F-Droid](https://f-droid.org/en/packages/com.termux/) (recommended) or GitHub.

### 2. Install dependencies
```bash
pkg update && pkg upgrade
pkg install python git golang
```

### 3. Clone & Run
```bash
git clone https://github.com/mmojoj101-del/bingdork-pro-termux
cd bingdork-pro-termux
make build
python3 guis/bingdork_termux.py
```

## 🎯 Features

| Feature | Description |
|---------|-------------|
| 🔍 **Single Search** | Quick dork queries |
| 📋 **Batch Scan** | Load 1000s of dorks from file |
| 📜 **History** | Previous searches saved |
| 📊 **Results Viewer** | Browse and export results |
| 🩺 **Diagnostics** | System health check |
| ⚙️ **Settings** | Provider, delay, format |
| 🎯 **20 Quick Dorks** | Pre-built examples |
| 🌈 **Colored TUI** | Beautiful terminal interface |
| 🔐 **CAPTCHA Bypass** | Auto-solving, session reuse |
| 🛡️ **Anti-Bot** | UA rotation, fingerprint randomization |
| 🔄 **Multi-Provider** | Bing, Google, DuckDuckGo, Brave |

## 🖼️ Screenshots

```
╔═══════════════════════════════════════╗
║  BingDork Pro v1.0.0                 ║
║  └─ الواجهة الرئيسية • Main Menu      ║
╚═══════════════════════════════════════╝

  ✓ Binary | Provider: bing | Delay: 2s

  [1] Single Search 🔍
  [2] Batch Search 📋
  [3] Queries File 📂
  [4] History 📜
  [5] Results 📊
  [6] Diagnostics 🩺
  [7] Settings ⚙️
  [8] Quick Dorks 🎯
  [0] Exit 🚪
```

## 📋 Dork Example File

Create `dorks.txt`:
```
site:example.com intitle:admin
site:example.com inurl:login
site:example.com filetype:pdf
intitle:index.of site:example.com
site:*.example.com
```

Then run:
```bash
python3 guis/bingdork_termux.py
# Select option 2 → Batch Search
```

## 🔧 CLI Direct (no GUI)

```bash
./bingdork search "site:example.com admin"
./bingdork batch -f dorks.txt -d 2 -t csv
./bingdork doctor
```

## 📁 Structure

```
bingdork-pro-termux/
├── guis/
│   └── bingdork_termux.py    ← Termux TUI (no dependencies!)
├── cli/                       ← Command-line interface
├── pkg/                       ← Core packages
├── internal/                  ← Internal modules
├── cmd/                       ← Entry point
├── docs/                      ← Documentation
└── ...
```

## 🔋 No External Dependencies

The Termux GUI uses **only Python standard library** - no pip installs needed!

## 📄 License

MIT License
