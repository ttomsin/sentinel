#!/usr/bin/env bash
# =============================================================================
# Sentinel — Install Script
# https://github.com/ttomsin/sentinel
#
# Usage:
#   curl -sSL https://raw.githubusercontent.com/ttomsin/sentinel/main/install.sh | bash
#
# What this does:
#   1. Checks Go is installed (1.22+)
#   2. Clones the Sentinel repo to a temp directory
#   3. Builds the binary
#   4. Installs to /usr/local/bin/sentinel (or ~/bin/sentinel if no sudo)
#   5. Verifies the install
#   6. Cleans up
# =============================================================================

set -e  # exit on any error

# ─── Colours ─────────────────────────────────────────────────────────────────

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
RESET='\033[0m'

# ─── Config ───────────────────────────────────────────────────────────────────

REPO="https://github.com/ttomsin/sentinel.git"
BINARY_NAME="sentinel"
MIN_GO_VERSION="1.22"
INSTALL_DIR="/usr/local/bin"
FALLBACK_DIR="$HOME/bin"

# ─── Helpers ─────────────────────────────────────────────────────────────────

print_banner() {
  echo ""
  echo -e "${YELLOW}${BOLD}"
  echo "  ███████╗███████╗███╗   ██╗████████╗██╗███╗   ██╗███████╗██╗"
  echo "  ██╔════╝██╔════╝████╗  ██║╚══██╔══╝██║████╗  ██║██╔════╝██║"
  echo "  ███████╗█████╗  ██╔██╗ ██║   ██║   ██║██╔██╗ ██║█████╗  ██║"
  echo "  ╚════██║██╔══╝  ██║╚██╗██║   ██║   ██║██║╚██╗██║██╔══╝  ██║"
  echo "  ███████║███████╗██║ ╚████║   ██║   ██║██║ ╚████║███████╗███████╗"
  echo "  ╚══════╝╚══════╝╚═╝  ╚═══╝   ╚═╝   ╚═╝╚═╝  ╚═══╝╚══════╝╚══════╝"
  echo -e "${RESET}"
  echo -e "  ${BOLD}Your code. Your rights. Protected.${RESET}"
  echo ""
  echo -e "  ${CYAN}https://github.com/ttomsin/sentinel${RESET}"
  echo ""
}

step() {
  echo -e "  ${YELLOW}→${RESET} $1"
}

success() {
  echo -e "  ${GREEN}✓${RESET} $1"
}

fail() {
  echo -e "  ${RED}✗ ERROR:${RESET} $1"
  echo ""
  exit 1
}

warn() {
  echo -e "  ${YELLOW}⚠${RESET}  $1"
}

# ─── Version comparison ───────────────────────────────────────────────────────

version_gte() {
  # Returns 0 (true) if $1 >= $2
  printf '%s\n%s\n' "$2" "$1" | sort -C -V
}

# ─── Main ─────────────────────────────────────────────────────────────────────

print_banner

echo -e "  ${BOLD}Installing Sentinel...${RESET}"
echo ""

# ── Step 1: Check Go is installed ─────────────────────────────────────────────

step "Checking Go installation..."

if ! command -v go &> /dev/null; then
  fail "Go is not installed.\n\n  Please install Go 1.22+ from https://go.dev/dl/\n  Then re-run this script."
fi

GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')

if ! version_gte "$GO_VERSION" "$MIN_GO_VERSION"; then
  fail "Go $GO_VERSION found but Sentinel requires Go $MIN_GO_VERSION+.\n  Please upgrade Go at https://go.dev/dl/"
fi

success "Go $GO_VERSION found."

# ── Step 2: Check Git is installed ────────────────────────────────────────────

step "Checking Git installation..."

if ! command -v git &> /dev/null; then
  fail "Git is not installed.\n  Please install Git from https://git-scm.com/"
fi

success "Git $(git --version | awk '{print $3}') found."

# ── Step 3: Clone to temp directory ───────────────────────────────────────────

TMPDIR=$(mktemp -d)
trap "rm -rf $TMPDIR" EXIT  # clean up temp dir on exit no matter what

step "Cloning Sentinel repository..."

if ! git clone --depth=1 "$REPO" "$TMPDIR/sentinel" &> /dev/null; then
  fail "Failed to clone repository from $REPO\n  Check your internet connection."
fi

success "Repository cloned."

# ── Step 4: Build the binary ──────────────────────────────────────────────────

step "Building Sentinel binary (go build)..."

cd "$TMPDIR/sentinel"

if ! go mod tidy &> /dev/null; then
  fail "go mod tidy failed — dependency issue."
fi

if ! go build -o "$BINARY_NAME" -ldflags="-s -w" .; then
  fail "Build failed. Check your Go installation."
fi

success "Build successful."

# ── Step 5: Install the binary ────────────────────────────────────────────────

step "Installing to system..."

INSTALLED_TO=""

# Try /usr/local/bin first (requires sudo on most systems)
if [ -w "$INSTALL_DIR" ]; then
  # Directory is writable — no sudo needed
  mv "$BINARY_NAME" "$INSTALL_DIR/$BINARY_NAME"
  INSTALLED_TO="$INSTALL_DIR/$BINARY_NAME"
elif command -v sudo &> /dev/null; then
  # Try with sudo
  echo ""
  warn "Installing to $INSTALL_DIR requires sudo password:"
  echo ""
  if sudo mv "$BINARY_NAME" "$INSTALL_DIR/$BINARY_NAME"; then
    INSTALLED_TO="$INSTALL_DIR/$BINARY_NAME"
  fi
fi

# Fallback to ~/bin if /usr/local/bin failed
if [ -z "$INSTALLED_TO" ]; then
  warn "Could not install to $INSTALL_DIR — installing to $FALLBACK_DIR instead."
  mkdir -p "$FALLBACK_DIR"
  mv "$BINARY_NAME" "$FALLBACK_DIR/$BINARY_NAME"
  INSTALLED_TO="$FALLBACK_DIR/$BINARY_NAME"

  # Check if ~/bin is in PATH
  if [[ ":$PATH:" != *":$FALLBACK_DIR:"* ]]; then
    echo ""
    warn "$FALLBACK_DIR is not in your PATH."
    echo ""
    echo "  Add this to your ~/.bashrc or ~/.zshrc:"
    echo -e "  ${CYAN}export PATH=\"\$HOME/bin:\$PATH\"${RESET}"
    echo ""
    echo "  Then reload your shell:"
    echo -e "  ${CYAN}source ~/.bashrc${RESET}  (or ~/.zshrc)"
    echo ""
  fi
fi

success "Installed to $INSTALLED_TO"

# ── Step 6: Verify install ────────────────────────────────────────────────────

step "Verifying installation..."

if command -v sentinel &> /dev/null; then
  INSTALLED_VERSION=$(sentinel --version 2>/dev/null || echo "Sentinel")
  success "sentinel command is available."
else
  warn "sentinel not found in PATH yet."
  echo "  You may need to restart your terminal or reload your shell config."
fi

# ── Done ──────────────────────────────────────────────────────────────────────

echo ""
echo -e "  ${GREEN}${BOLD}✓ Sentinel installed successfully!${RESET}"
echo ""
echo "  ─────────────────────────────────────────"
echo "  Get started:"
echo ""
echo -e "  ${CYAN}cd your-project${RESET}"
echo -e "  ${CYAN}sentinel init${RESET}       # initialize in your repo"
echo -e "  ${CYAN}sentinel keygen${RESET}     # generate your encryption keys"
echo -e "  ${CYAN}sentinel commit -m \"first protected commit\"${RESET}"
echo ""
echo "  Full documentation:"
echo -e "  ${CYAN}https://github.com/ttomsin/sentinel${RESET}"
echo ""
echo -e "  ${BOLD}Your code. Your rights. Protected.${RESET}"
echo ""
