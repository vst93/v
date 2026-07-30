#!/bin/bash
#
# v installer
#
#   /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/vst93/v/refs/heads/main/cmd/install.sh)"
#
# Environment variables:
#   INSTALL_DIR   Target directory (default: first writable of /usr/local/bin,
#                 ~/.local/bin, ~/bin)
#   VERSION       Release tag to install (default: latest)
#   FORCE=1       Reinstall even when the installed version already matches

set -euo pipefail

REPO_URL="https://github.com/vst93/v"
API_URL="https://api.github.com/repos/vst93/v"
BINARY_NAME="v"
TEMP_DIR=$(mktemp -d)

INSTALL_DIR="${INSTALL_DIR:-}"
VERSION="${VERSION:-}"
FORCE="${FORCE:-0}"

# All progress output goes to stderr so that functions can use stdout to
# return values through $(...) without the logs being captured too.
log()  { echo "$*" >&2; }
die()  { echo "Error: $*" >&2; exit 1; }

cleanup() { rm -rf "$TEMP_DIR"; }
trap cleanup EXIT

# ---------------------------------------------------------------------------
# Download helpers
# ---------------------------------------------------------------------------

have() { command -v "$1" >/dev/null 2>&1; }

fetch_stdout() {
    local url="$1"
    if have curl; then
        curl -fsSL "$url"
    elif have wget; then
        wget -qO- "$url"
    else
        die "curl or wget is required"
    fi
}

download_file() {
    local url="$1" output_file="$2"

    if have curl; then
        curl -fsSL -o "$output_file" "$url"
    elif have wget; then
        wget -qO "$output_file" "$url"
    else
        die "curl or wget is required for downloading"
    fi

    if [ ! -s "$output_file" ]; then
        die "download failed or file is empty: $url"
    fi
}

# ---------------------------------------------------------------------------
# Platform detection
# ---------------------------------------------------------------------------

# uname -o is not portable (older macOS BSD uname rejects it outright), so the
# primary switch is on uname -s.
detect_platform() {
    local kernel
    kernel="$(uname -s)"

    case "$kernel" in
        Darwin)
            OS="darwin"
            ;;
        Linux)
            # Termux on Android reports Linux; $PREFIX is the reliable tell.
            if [ -n "${PREFIX:-}" ] && [ -d "${PREFIX}/bin" ]; then
                OS="android"
            elif [ "$(uname -o 2>/dev/null || echo)" = "Android" ]; then
                OS="android"
            else
                OS="linux"
            fi
            ;;
        MINGW*|MSYS*|CYGWIN*)
            OS="windows"
            ;;
        *)
            die "unsupported OS: $kernel"
            ;;
    esac

    case "$(uname -m)" in
        x86_64|amd64)  ARCH="amd64" ;;
        arm64|aarch64) ARCH="arm64" ;;
        *) die "unsupported CPU architecture: $(uname -m)" ;;
    esac

    log "Detected platform: $OS-$ARCH"
}

# The release matrix is linux/darwin/android x amd64/arm64, plus windows/amd64.
get_download_info() {
    local os="$1" arch="$2"

    case "$os-$arch" in
        darwin-arm64|darwin-amd64|linux-arm64|linux-amd64|android-arm64|android-amd64|windows-amd64)
            FILENAME="${BINARY_NAME}-${os}-${arch}.zip"
            ;;
        *)
            die "no package available for $os-$arch"
            ;;
    esac

    BINARY_FILE="$BINARY_NAME"
    if [ "$os" = "windows" ]; then
        BINARY_FILE="${BINARY_NAME}.exe"
    fi

    DOWNLOAD_URL="${REPO_URL}/releases/download/${VERSION}/${FILENAME}"
}

# ---------------------------------------------------------------------------
# Version resolution
# ---------------------------------------------------------------------------

resolve_latest_version() {
    local response tag
    response="$(fetch_stdout "${API_URL}/releases/latest")" \
        || die "failed to reach the GitHub API (rate limited? try VERSION=<tag>)"
    tag="$(echo "$response" | grep '"tag_name"' | head -1 | cut -d'"' -f4)"
    [ -n "$tag" ] && echo "$tag"
}

installed_version() {
    have "$BINARY_NAME" || return 0
    "$BINARY_NAME" -version 2>/dev/null | head -1 | tr -d '[:space:]' || true
}

# ---------------------------------------------------------------------------
# Install directory
# ---------------------------------------------------------------------------

determine_install_dir() {
    if [ -n "$INSTALL_DIR" ]; then
        log "Using user-specified install directory: $INSTALL_DIR"
        echo "$INSTALL_DIR"
        return 0
    fi

    local candidate
    for candidate in /usr/local/bin "$HOME/.local/bin" "$HOME/bin"; do
        if [ -d "$candidate" ] && [ -w "$candidate" ]; then
            echo "$candidate"
            return 0
        fi
    done

    # Nothing writable exists yet: create a user-owned directory.
    for candidate in "$HOME/.local/bin" "$HOME/bin"; do
        if mkdir -p "$candidate" 2>/dev/null; then
            echo "$candidate"
            return 0
        fi
    done

    # Fall back to /usr/local/bin and let the install step escalate.
    echo "/usr/local/bin"
}

# Prints "sudo" when writing to dir needs elevation, empty otherwise.
sudo_prefix_for() {
    local dir="$1"

    if [ ! -d "$dir" ]; then
        if mkdir -p "$dir" 2>/dev/null; then
            echo ""
            return 0
        fi
        have sudo || die "cannot create $dir and sudo is unavailable"
        log "Creating $dir (needs sudo)"
        sudo mkdir -p "$dir" || die "cannot create directory: $dir"
        echo "sudo"
        return 0
    fi

    if [ -w "$dir" ]; then
        echo ""
    else
        have sudo || die "$dir is not writable and sudo is unavailable"
        echo "sudo"
    fi
}

# ---------------------------------------------------------------------------
# SHA256
# ---------------------------------------------------------------------------

sha256_of() {
    local file="$1"
    if have shasum; then
        shasum -a 256 "$file" | cut -d ' ' -f1
    elif have sha256sum; then
        sha256sum "$file" | cut -d ' ' -f1
    fi
}

verify_sha256() {
    local file="$1" sha256_url="$2"

    if ! have shasum && ! have sha256sum; then
        log "Warning: shasum/sha256sum not found, skipping verification"
        return 0
    fi

    local sha_file="$TEMP_DIR/expected.sha256"
    if ! download_file "$sha256_url" "$sha_file" 2>/dev/null; then
        log "Warning: cannot fetch SHA256 checksum, skipping verification"
        return 0
    fi

    local expected actual
    expected="$(tr -d '[:space:]' < "$sha_file" | cut -d ' ' -f1)"
    actual="$(sha256_of "$file")"

    if [ "$actual" != "$expected" ]; then
        log "Expected: $expected"
        log "Actual:   $actual"
        die "SHA256 verification failed, aborting installation"
    fi
    log "SHA256 verification passed"
}

# ---------------------------------------------------------------------------
# Install
# ---------------------------------------------------------------------------

install_binary() {
    local zip_file="$1" install_dir="$2"

    have unzip || die "unzip is required (Debian/Ubuntu: sudo apt install unzip, macOS: brew install unzip, Termux: pkg install unzip)"

    log "Extracting..."
    unzip -q -o "$zip_file" -d "$TEMP_DIR"

    local binary_path="$TEMP_DIR/$BINARY_FILE"
    [ -f "$binary_path" ] || die "cannot find $BINARY_FILE in the archive"
    chmod +x "$binary_path"

    local sudo_cmd
    sudo_cmd="$(sudo_prefix_for "$install_dir")"

    log "Installing to $install_dir"
    $sudo_cmd mv "$binary_path" "$install_dir/$BINARY_FILE"
}

suggest_path() {
    local install_dir="$1" shell_rc=""

    # Pick the rc file for the shell actually in use, not whichever file
    # happens to exist first.
    case "$(basename "${SHELL:-}")" in
        zsh)  shell_rc="$HOME/.zshrc" ;;
        bash) shell_rc="$HOME/.bashrc" ;;
        fish) shell_rc="$HOME/.config/fish/config.fish" ;;
        *)    shell_rc="$HOME/.profile" ;;
    esac

    log ""
    log "Note: $install_dir is not in your PATH."
    if [ "$(basename "${SHELL:-}")" = "fish" ]; then
        log "  fish_add_path $install_dir"
    else
        log "  echo 'export PATH=\"$install_dir:\$PATH\"' >> $shell_rc"
        log "  source $shell_rc"
    fi
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------

main() {
    if [ -z "$VERSION" ]; then
        log "Resolving latest release..."
        VERSION="$(resolve_latest_version)"
        [ -n "$VERSION" ] || die "failed to get the latest release version from the GitHub API"
    fi

    local current
    current="$(installed_version)"

    if [ -n "$current" ]; then
        # Tags may or may not carry a leading "v"; the binary never reports one.
        if [ "$current" = "${VERSION#v}" ] && [ "$FORCE" != "1" ]; then
            log "$BINARY_NAME $current is already the latest version, nothing to do."
            log "(Reinstall anyway with: FORCE=1)"
            return 0
        fi
        log "Upgrading $BINARY_NAME $current -> ${VERSION#v}"
    else
        log "Installing $BINARY_NAME ${VERSION#v}"
    fi

    detect_platform
    get_download_info "$OS" "$ARCH"

    INSTALL_DIR="$(determine_install_dir)"
    log "Installation directory: $INSTALL_DIR"
    log "Download URL: $DOWNLOAD_URL"

    local zip_file="$TEMP_DIR/$FILENAME"
    download_file "$DOWNLOAD_URL" "$zip_file"
    verify_sha256 "$zip_file" "${DOWNLOAD_URL}.sha256"

    install_binary "$zip_file" "$INSTALL_DIR"

    log ""
    if have "$BINARY_NAME"; then
        log "✅ Installed $BINARY_NAME $("$BINARY_NAME" -version 2>/dev/null || echo "${VERSION#v}")"
        log "Get started with: $BINARY_NAME -h"
    else
        log "✅ Installed to $INSTALL_DIR/$BINARY_FILE"
        suggest_path "$INSTALL_DIR"
    fi
}

main "$@"
