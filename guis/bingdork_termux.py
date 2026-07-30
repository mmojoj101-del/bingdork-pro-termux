#!/usr/bin/env python3
"""
BingDork Pro - Termux Android TUI
Terminal User Interface for Android via Termux

Requirements:
    pkg install python
    pip install requests

Usage:
    chmod +x guis/bingdork_termux.py
    ./guis/bingdork_termux.py

    OR on phone:
    pkg install python git
    git clone https://github.com/bingdork/bingdork
    cd bingdork
    python guis/bingdork_termux.py
"""

import os
import sys
import json
import csv
import subprocess
import shutil
import time
import shutil
from datetime import datetime
from pathlib import Path
from typing import List, Dict, Optional

# ======== Terminal Colors ========
if os.name == 'nt':
    # Windows
    RED = YELLOW = GREEN = CYAN = WHITE = BOLD = RESET = ''
    BLUE = MAGENTA = ''
else:
    RED = '\033[91m'
    YELLOW = '\033[93m'
    GREEN = '\033[92m'
    CYAN = '\033[96m'
    BLUE = '\033[94m'
    MAGENTA = '\033[95m'
    WHITE = '\033[97m'
    BOLD = '\033[1m'
    DIM = '\033[2m'
    RESET = '\033[0m'
    CLEAR = '\033[2J\033[H'

# ======== Constants ========
APP_NAME = "BingDork Pro"
APP_VERSION = "1.0.0"
BINARY_PATH = None

# Find binary
for path in [
    "./build/bingdork", "./bingdork", "/tmp/bingdork",
    "/usr/local/bin/bingdork", os.path.expanduser("~/go/bin/bingdork"),
    "/data/data/com.termux/files/usr/bin/bingdork",
]:
    if os.path.isfile(path) and os.access(path, os.X_OK):
        BINARY_PATH = os.path.abspath(path)
        break

HISTORY_FILE = os.path.expanduser("~/.bingdork_termux_history.json")
CONFIG_FILE = os.path.expanduser("~/.bingdork_termux_config.json")


# ======== Terminal Helpers ========
def clear_screen():
    """Clear terminal screen."""
    if os.name == 'nt':
        os.system('cls')
    else:
        os.system('clear')


def print_header(title: str = ""):
    """Print app header."""
    width = shutil.get_terminal_size().columns
    line = "═" * width

    print(f"{BOLD}{CYAN}{line}{RESET}")
    print(f"{BOLD}{CYAN}  {APP_NAME} v{APP_VERSION}{RESET}")
    if title:
        print(f"{BOLD}{WHITE}  └─ {title}{RESET}")
    print(f"{BOLD}{CYAN}{line}{RESET}")
    print()


def print_menu(items: List[tuple]):
    """Print numbered menu items."""
    for i, (key, label, desc) in enumerate(items, 1):
        print(f"  {BOLD}{GREEN}[{key}]{RESET} {WHITE}{label}{RESET}")
        if desc:
            print(f"       {DIM}{desc}{RESET}")
    print()


def print_info(label: str, value: str, color: str = WHITE):
    """Print info line."""
    print(f"  {BOLD}{label}:{RESET} {color}{value}{RESET}")


def print_success(msg: str):
    """Print success message."""
    print(f"  {BOLD}{GREEN}✅ {msg}{RESET}")


def print_error(msg: str):
    """Print error message."""
    print(f"  {BOLD}{RED}❌ {msg}{RESET}")


def print_warning(msg: str):
    """Print warning message."""
    print(f"  {BOLD}{YELLOW}⚠️ {msg}{RESET}")


def print_info_msg(msg: str):
    """Print info message."""
    print(f"  {BOLD}{BLUE}ℹ️ {msg}{RESET}")


def input_str(prompt: str, default: str = "") -> str:
    """Get string input with prompt."""
    if default:
        val = input(f"  {prompt} [{default}]: ").strip()
        return val if val else default
    return input(f"  {prompt}: ").strip()


def input_int(prompt: str, default: int = 0) -> int:
    """Get integer input."""
    val = input(f"  {prompt} [{default}]: ").strip()
    if not val:
        return default
    try:
        return int(val)
    except ValueError:
        return default


def input_choice(prompt: str, options: List[str], default: str = "") -> str:
    """Get a choice from options."""
    print(f"  {prompt}:")
    for i, opt in enumerate(options, 1):
        print(f"    {i}. {opt}")
    val = input(f"  Choice [{default or '1'}]: ").strip()
    if not val:
        return default if default else options[0]
    try:
        idx = int(val) - 1
        if 0 <= idx < len(options):
            return options[idx]
    except ValueError:
        pass
    return val if val in options else (default or options[0])


def press_enter():
    """Wait for Enter key."""
    input(f"\n  {DIM}Press Enter to continue...{RESET}")


# ======== Configuration ========
def load_config() -> dict:
    """Load saved configuration."""
    cfg = {
        "provider": "bing",
        "delay": 2,
        "format": "json",
        "max_results": 0,
        "history_size": 20,
    }
    if os.path.isfile(CONFIG_FILE):
        try:
            with open(CONFIG_FILE, 'r') as f:
                cfg.update(json.load(f))
        except:
            pass
    return cfg


def save_config(cfg: dict):
    """Save configuration."""
    os.makedirs(os.path.dirname(CONFIG_FILE) or '.', exist_ok=True)
    with open(CONFIG_FILE, 'w') as f:
        json.dump(cfg, f, indent=2)


# ======== History ========
def load_history() -> List[str]:
    """Load search history."""
    if os.path.isfile(HISTORY_FILE):
        try:
            with open(HISTORY_FILE, 'r') as f:
                return json.load(f)
        except:
            pass
    return []


def save_history(history: List[str], max_size: int = 50):
    """Save search history."""
    history = history[-max_size:]
    os.makedirs(os.path.dirname(HISTORY_FILE) or '.', exist_ok=True)
    with open(HISTORY_FILE, 'w') as f:
        json.dump(history, f, indent=2)


def add_to_history(query: str, history: List[str], max_size: int = 50):
    """Add query to history."""
    if query in history:
        history.remove(query)
    history.append(query)
    save_history(history, max_size)


# ======== Dork Runner ========
class DorkRunner:
    """Handles executing bingdork commands."""
    
    def __init__(self):
        self.running = False
        self.last_output = ""
        self.last_results = []

    def check_binary(self) -> bool:
        """Check if bingdork binary is available."""
        global BINARY_PATH
        if BINARY_PATH:
            return True
        # Try to find it
        result = subprocess.run(
            ["which", "bingdork"], capture_output=True, text=True
        )
        if result.returncode == 0:
            BINARY_PATH = result.stdout.strip()
            return True
        return False

    def run_command(self, cmd: List[str], show_output: bool = True) -> int:
        """Run a bingdork command and return exit code."""
        if not self.check_binary():
            print_error("BingDork binary not found!")
            print_info_msg("Install: cd bingdork && make build")
            print_info_msg("Or: cp build/bingdork /data/data/com.termux/files/usr/bin/")
            return -1

        self.running = True
        full_cmd = [BINARY_PATH] + cmd
        output_lines = []

        try:
            if show_output:
                print(f"\n  {DIM}Running: {' '.join(full_cmd)}{RESET}\n")

            process = subprocess.Popen(
                full_cmd,
                stdout=subprocess.PIPE,
                stderr=subprocess.STDOUT,
                text=True,
                bufsize=1,
            )

            for line in iter(process.stdout.readline, ''):
                line = line.rstrip()
                if line:
                    output_lines.append(line)
                    if show_output:
                        print(f"  {DIM}{line}{RESET}")

            process.stdout.close()
            process.wait()
            self.running = False
            self.last_output = '\n'.join(output_lines)

            # Try to parse results
            try:
                data = json.loads(self.last_output)
                if isinstance(data, dict):
                    self.last_results = data.get("results", [])
                elif isinstance(data, list):
                    self.last_results = data
            except:
                self.last_results = []

            if process.returncode == 0:
                print()
                print_success("Command completed successfully")
            else:
                print()
                print_error(f"Command failed with code {process.returncode}")

            return process.returncode

        except Exception as e:
            self.running = False
            print_error(f"Error: {e}")
            return -1

    def search(self, query: str, provider: str = "bing",
               fmt: str = "json", page: int = 0) -> int:
        """Execute a single search."""
        cmd = ["search"]
        if provider:
            cmd.extend(["-p", provider])
        if fmt:
            cmd.extend(["-f", fmt])
        if page > 0:
            cmd.extend(["-n", str(page)])
        cmd.append(query)
        return self.run_command(cmd)

    def batch(self, filepath: str, provider: str = "bing",
              delay: int = 2, fmt: str = "json") -> int:
        """Execute batch search."""
        if not os.path.isfile(filepath):
            print_error(f"File not found: {filepath}")
            return -1
        cmd = ["batch", "-f", filepath]
        if provider:
            cmd.extend(["-p", provider])
        if delay > 0:
            cmd.extend(["-d", str(delay)])
        if fmt:
            cmd.extend(["-t", fmt])
        return self.run_command(cmd)

    def doctor(self) -> int:
        """Run diagnostics."""
        return self.run_command(["doctor"])

    def stats(self) -> int:
        """Show statistics."""
        return self.run_command(["stats"])


# ======== UI Screens ========
class App:
    """Main application."""
    
    def __init__(self):
        self.config = load_config()
        self.history = load_history()
        self.runner = DorkRunner()
        self.queries_file = ""

    def run(self):
        """Main application loop."""
        while True:
            clear_screen()
            print_header("الواجهة الرئيسية • Main Menu")

            self._show_status()

            print_menu([
                ("1", "Single Search 🔍", "Execute a dork search"),
                ("2", "Batch Search 📋", "Multiple queries from file"),
                ("3", "Queries File 📂", "Create/edit queries file"),
                ("4", "History 📜", "Previous searches"),
                ("5", "Results 📊", "View last results"),
                ("6", "Diagnostics 🩺", "System health check"),
                ("7", "Settings ⚙️", "Configuration"),
                ("8", "Quick Dorks 🎯", "Pre-built dork examples"),
                ("0", "Exit 🚪", "Quit application"),
            ])

            choice = input(f"  {BOLD}Enter choice{RESET}: ").strip()

            handlers = {
                "1": self.screen_search,
                "2": self.screen_batch,
                "3": self.screen_queries_file,
                "4": self.screen_history,
                "5": self.screen_results,
                "6": self.screen_doctor,
                "7": self.screen_settings,
                "8": self.screen_quick_dorks,
                "0": self.screen_exit,
            }

            handler = handlers.get(choice)
            if handler:
                handler()
            else:
                print_error("Invalid choice!")
                press_enter()

    def _show_status(self):
        """Show status bar."""
        binary_status = f"{GREEN}✓{RESET}" if BINARY_PATH else f"{RED}✗{RESET}"
        print(f"  {DIM}Binary: {binary_status} | "
              f"Provider: {self.config['provider']} | "
              f"Delay: {self.config['delay']}s{RESET}")
        print()

    # ----- Screen: Search -----
    def screen_search(self):
        """Single search screen."""
        clear_screen()
        print_header("🔍 Single Search")

        # Show recent history
        if self.history:
            print(f"  {DIM}Recent queries:{RESET}")
            for i, q in enumerate(reversed(self.history[-5:]), 1):
                print(f"    {i}. {DIM}{q[:80]}{RESET}")
            print()

        query = input_str("Enter dork query")
        if not query:
            return

        # Use first history item if just number
        if query.isdigit() and self.history:
            idx = int(query) - 1
            if 0 <= idx < len(self.history):
                query = self.history[-(idx+1)]
                print(f"  Using: {CYAN}{query}{RESET}")

        provider = input_choice("Provider",
            ["bing", "google", "duckduckgo", "brave"],
            self.config['provider'])
        fmt = input_choice("Format",
            ["json", "csv", "txt", "md"], self.config['format'])

        add_to_history(query, self.history, self.config['history_size'])

        print()
        self.runner.search(query, provider, fmt)
        press_enter()

    # ----- Screen: Batch -----
    def screen_batch(self):
        """Batch search screen."""
        clear_screen()
        print_header("📋 Batch Search")

        if not self.queries_file or not os.path.isfile(self.queries_file):
            print_warning("No queries file loaded!")
            print_info_msg("Use option 3 to create or select a file first")
            press_enter()
            return

        # Count queries
        count = 0
        with open(self.queries_file, 'r') as f:
            for line in f:
                line = line.strip()
                if line and not line.startswith('#'):
                    count += 1

        print_info("File", self.queries_file)
        print_info("Queries", str(count))
        print()

        if count == 0:
            print_warning("No queries found in file!")
            press_enter()
            return

        provider = input_choice("Provider",
            ["bing", "google", "duckduckgo", "brave"],
            self.config['provider'])
        delay = input_int("Delay (seconds)", self.config['delay'])
        fmt = input_choice("Format",
            ["json", "csv", "txt", "md"], self.config['format'])

        estimated = count * delay
        print()
        print_warning(f"Estimated time: ~{estimated} seconds ({estimated//60}m {estimated%60}s)")
        confirm = input(f"  {BOLD}Start batch? (y/N): {RESET}").strip().lower()
        if confirm != 'y':
            return

        self.runner.batch(self.queries_file, provider, delay, fmt)
        press_enter()

    # ----- Screen: Queries File -----
    def screen_queries_file(self):
        """Queries file management."""
        clear_screen()
        print_header("📂 Queries File")

        print_menu([
            ("1", "Load file", "Select existing queries file"),
            ("2", "Create file", "Create new queries file"),
            ("3", "Edit file", "Open in editor"),
            ("4", "View file", "Show contents"),
            ("5", "Count queries", "Count dorks in file"),
            ("b", "Back ↩", "Return to main menu"),
        ])

        choice = input("  Enter choice: ").strip()

        if choice == "1":
            path = input_str("Path to queries file")
            if os.path.isfile(path):
                self.queries_file = os.path.abspath(path)
                print_success(f"Loaded: {self.queries_file}")
            else:
                print_error(f"File not found: {path}")
            press_enter()

        elif choice == "2":
            path = input_str("Save as", "dorks.txt")
            print_info_msg("Enter one dork per line. End with Ctrl+D or empty line.")
            print()
            queries = []
            try:
                while True:
                    line = input("  ")
                    if not line:
                        break
                    queries.append(line)
            except EOFError:
                pass

            if queries:
                with open(path, 'w') as f:
                    for q in queries:
                        f.write(q + '\n')
                self.queries_file = os.path.abspath(path)
                print_success(f"Saved {len(queries)} queries to {path}")
            else:
                print_warning("No queries entered!")
            press_enter()

        elif choice == "3":
            if not self.queries_file:
                print_warning("No file loaded!")
                press_enter()
                return
            editor = os.environ.get('EDITOR', 'nano')
            subprocess.run([editor, self.queries_file])

        elif choice == "4":
            if not self.queries_file or not os.path.isfile(self.queries_file):
                print_warning("No file loaded!")
                press_enter()
                return
            print()
            with open(self.queries_file, 'r') as f:
                for line in f:
                    line = line.rstrip()
                    if line:
                        print(f"  {DIM}{line[:100]}{RESET}")
            press_enter()

        elif choice == "5":
            if not self.queries_file or not os.path.isfile(self.queries_file):
                print_warning("No file loaded!")
                press_enter()
                return
            count = 0
            with open(self.queries_file, 'r') as f:
                for line in f:
                    if line.strip() and not line.startswith('#'):
                        count += 1
            print_info("Total queries", str(count))
            press_enter()

    # ----- Screen: History -----
    def screen_history(self):
        """Search history screen."""
        clear_screen()
        print_header("📜 Search History")

        if not self.history:
            print_info_msg("No history yet!")
            press_enter()
            return

        for i, q in enumerate(reversed(self.history), 1):
            print(f"  {BOLD}{GREEN}{i:3}.{RESET} {q[:100]}")

        print()
        choice = input_str("Run query by number, or Enter to go back")
        if choice and choice.isdigit():
            idx = int(choice) - 1
            if 0 <= idx < len(self.history):
                query = self.history[-(idx+1)]
                print()
                self.runner.search(query, self.config['provider'], self.config['format'])
                press_enter()
            else:
                print_error("Invalid number!")
                press_enter()

    # ----- Screen: Results -----
    def screen_results(self):
        """View last results."""
        clear_screen()
        print_header("📊 Last Results")

        if not self.runner.last_results:
            print_info_msg("No results yet! Run a search first.")
            press_enter()
            return

        results = self.runner.last_results
        print_info("Total results", str(len(results)))
        print()

        for i, r in enumerate(results[:20], 1):
            if isinstance(r, dict):
                title = r.get('title', 'N/A')[:80]
                url = r.get('url', '')
                host = r.get('host', '')
                print(f"  {BOLD}{GREEN}{i:3}.{RESET} {title}")
                if url:
                    print(f"       {DIM}URL: {url[:100]}{RESET}")
                if host:
                    print(f"       {DIM}Host: {host}{RESET}")
                print()

        if len(results) > 20:
            print_info_msg(f"... and {len(results) - 20} more results")

        # Export option
        export = input("  Export results? (y/N): ").strip().lower()
        if export == 'y':
            path = input_str("Export path", "results.json")
            try:
                with open(path, 'w') as f:
                    json.dump(results, f, indent=2)
                print_success(f"Exported to {path}")
            except Exception as e:
                print_error(f"Export failed: {e}")
        press_enter()

    # ----- Screen: Doctor -----
    def screen_doctor(self):
        """Diagnostics screen."""
        clear_screen()
        print_header("🩺 Diagnostics")

        print_info("Binary", str(BINARY_PATH or "Not found"),
                   GREEN if BINARY_PATH else RED)
        print_info("Python", sys.version)
        print_info("Platform", sys.platform)
        print_info("CWD", os.getcwd())
        print_info("Config", CONFIG_FILE)
        print_info("History", HISTORY_FILE)
        print()

        # Test binary
        if BINARY_PATH:
            print_info_msg("Running diagnostics...")
            self.runner.doctor()
        else:
            print_error("BingDork binary not found!")
            print_info_msg("Install: cd bingdork && make build")

        print()
        print_info_msg("Environment:")
        for key, val in sorted(os.environ.items()):
            if 'BING' in key.upper() or 'DORK' in key.upper():
                print(f"    {key}={val}")

        press_enter()

    # ----- Screen: Settings -----
    def screen_settings(self):
        """Settings screen."""
        clear_screen()
        print_header("⚙️ Settings")

        print_menu([
            ("1", "Default Provider", f"Current: {self.config['provider']}"),
            ("2", "Delay Between Queries", f"Current: {self.config['delay']}s"),
            ("3", "Output Format", f"Current: {self.config['format']}"),
            ("4", "History Size", f"Current: {self.config['history_size']}"),
            ("5", "Save Config", "Save current settings"),
            ("6", "Reset Config", "Reset to defaults"),
            ("b", "Back ↩", "Return to main menu"),
        ])

        choice = input("  Enter choice: ").strip()

        if choice == "1":
            self.config['provider'] = input_choice("Default provider",
                ["bing", "google", "duckduckgo", "brave"],
                self.config['provider'])
            save_config(self.config)
            print_success("Saved!")

        elif choice == "2":
            self.config['delay'] = input_int("Delay (seconds)",
                self.config['delay'])
            save_config(self.config)
            print_success("Saved!")

        elif choice == "3":
            self.config['format'] = input_choice("Output format",
                ["json", "csv", "txt", "md"],
                self.config['format'])
            save_config(self.config)
            print_success("Saved!")

        elif choice == "4":
            self.config['history_size'] = input_int("History size",
                self.config['history_size'])
            save_config(self.config)
            print_success("Saved!")

        elif choice == "5":
            save_config(self.config)
            print_success("Configuration saved!")

        elif choice == "6":
            if input("  Reset to defaults? (y/N): ").strip().lower() == 'y':
                os.remove(CONFIG_FILE) if os.path.isfile(CONFIG_FILE) else None
                self.config = load_config()
                print_success("Configuration reset!")

        press_enter()

    # ----- Screen: Quick Dorks -----
    def screen_quick_dorks(self):
        """Quick dork examples."""
        clear_screen()
        print_header("🎯 Quick Dork Examples")

        dorks = [
            ("🌐 Site Scan", "site:example.com"),
            ("🔐 Admin Panels", "site:example.com intitle:admin"),
            ("📄 PDF Files", "site:example.com filetype:pdf"),
            ("🔑 Login Pages", "site:example.com inurl:login"),
            ("🏢 Subdomains", "site:*.example.com"),
            ("📂 Index Of", "intitle:index.of site:example.com"),
            ("🐘 PHP Info", "inurl:phpinfo.php"),
            ("📁 Git Repos", "inurl:.git"),
            ("🔌 API Endpoints", "site:example.com inurl:api"),
            ("💾 Backup Files", "site:example.com ext:bak OR ext:old"),
            ("🐍 Python Config", "ext:py inurl:config"),
            ("🛢️ SQL Dumps", "ext:sql intext:password"),
            ("🔧 Environment", "ext:env inurl:.env"),
            ("📊 Exposed DB", "ext:sqlite3 OR ext:db inurl:admin"),
            ("🔐 AWS Keys", "intext:AKIA[0-9A-Z]{16}"),
            ("🐳 Docker Files", "inurl:Dockerfile"),
            ("📝 Jenkins", "inurl:jenkins"),
            ("📋 Grafana", "inurl:grafana intitle:Grafana"),
            ("🔄 Jira", "inurl:jira intitle:dashboard"),
            ("📚 Wiki", "inurl:wiki intitle:index.of"),
        ]

        for i, (name, dork) in enumerate(dorks, 1):
            print(f"  {BOLD}{GREEN}{i:2}.{RESET} {WHITE}{name:25}{RESET} {CYAN}{dork}{RESET}")

        print(f"\n  {BOLD}{GREEN}a.{RESET} {WHITE}Run All 20 Dorks{RESET} {CYAN}(batch mode){RESET}")
        print(f"  {BOLD}{RED}b.{RESET} Back")

        choice = input(f"\n  {BOLD}Select dork{getattr(self, '_last_choice', '')}{RESET}: ").strip()

        if choice == 'b':
            return
        elif choice == 'a':
            # Run all dorks
            self._run_all_dorks(dorks)
        elif choice.isdigit():
            idx = int(choice) - 1
            if 0 <= idx < len(dorks):
                name, dork = dorks[idx]
                print(f"\n  Running: {CYAN}{dork}{RESET}")
                print()
                self.runner.search(dork, self.config['provider'], self.config['format'])
                press_enter()

    def _run_all_dorks(self, dorks: List[tuple]):
        """Run all quick dorks in batch mode."""
        tmpfile = "/tmp/quick_dorks.txt"
        with open(tmpfile, 'w') as f:
            for _, dork in dorks:
                f.write(dork + '\n')

        print_info_msg(f"Running {len(dorks)} dorks...")
        print_warning(f"Delay: {self.config['delay']}s per query")
        print()

        self.runner.batch(tmpfile, self.config['provider'],
                          self.config['delay'], self.config['format'])
        os.remove(tmpfile)
        press_enter()

    # ----- Screen: Exit -----
    def screen_exit(self):
        """Exit application."""
        print(f"\n  {BOLD}{GREEN}Goodbye!{RESET}\n")
        sys.exit(0)


# ======== Main Entry ========
def main():
    """Application entry point."""
    try:
        app = App()
        app.run()
    except KeyboardInterrupt:
        print(f"\n  {BOLD}{YELLOW}Interrupted{RESET}\n")
        sys.exit(0)
    except Exception as e:
        print(f"\n  {RED}Fatal error: {e}{RESET}\n")
        sys.exit(1)


if __name__ == "__main__":
    main()
