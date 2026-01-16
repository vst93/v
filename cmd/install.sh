#!/bin/bash

set -e

# Configuration
REPO="https://github.com/vst93/v"
BINARY_NAME="v"
TEMP_DIR=$(mktemp -d)

# Get latest release version from GitHub API
response=$(curl -s https://api.github.com/repos/vst93/$BINARY_NAME/releases/latest)
VERSION=$(echo "$response" | grep 'tag_name' | cut -d'"' -f4)

# Validate VERSION is not empty
if [ -z "$VERSION" ]; then
    echo "Error: Failed to get latest release version from GitHub API"
    exit 1
fi

# Determine installation directory
determine_install_dir() {
    local install_dir=""

    # 1. Check if user specified install directory
    if [ -n "$INSTALL_DIR" ]; then
        install_dir="$INSTALL_DIR"
        echo "Using user-specified install directory: $install_dir"
        echo "$install_dir"
        return 0
    fi

    # 2. Check /usr/local/bin
    if [ -d "/usr/local/bin" ] && [ -w "/usr/local/bin" ]; then
        echo "/usr/local/bin"
        return 0
    fi

    # 3. Check ~/.local/bin
    local user_local_bin="$HOME/.local/bin"
    if [ -n "$HOME" ] && [ -d "$user_local_bin" ]; then
        if [ -w "$user_local_bin" ]; then
            echo "$user_local_bin"
            return 0
        fi
    fi

    # 4. Check ~/bin
    local user_bin="$HOME/bin"
    if [ -n "$HOME" ] && [ -d "$user_bin" ]; then
        if [ -w "$user_bin" ]; then
            echo "$user_bin"
            return 0
        fi
    fi

    # 5. Try to create ~/.local/bin if it doesn't exist
    if [ -n "$HOME" ]; then
        local new_dir="$HOME/.local/bin"
        if mkdir -p "$new_dir" 2>/dev/null; then
            echo "$new_dir"
            return 0
        fi
    fi

    # 6. Try to create ~/bin if it doesn't exist
    if [ -n "$HOME" ]; then
        local new_dir="$HOME/bin"
        if mkdir -p "$new_dir" 2>/dev/null; then
            echo "$new_dir"
            return 0
        fi
    fi

    echo "Error: Cannot determine installation directory"
    exit 1
}

# Cleanup function
cleanup() {
    rm -rf "$TEMP_DIR"
}

# Error handling
trap cleanup EXIT
trap 'echo "Error occurred during installation"; exit 1' ERR

# Detect platform and architecture
detect_platform() {
    OS=""
    ARCH=""

    # Detect OS
    case "$(uname -o)" in
        Darwin)
            OS="darwin"
            ;;
        GNU/Linux)
            OS="linux"
            ;;
        Android)
            OS="android"
            ;;
        *)
            echo "Unsupported OS: $(uname -o)"
            exit 1
            ;;
    esac

    # Detect CPU architecture
    case "$(uname -m)" in
        x86_64|amd64)
            ARCH="amd64"
            ;;
        arm64|aarch64)
            ARCH="arm64"
            ;;
        *)
            echo "Unsupported CPU architecture: $(uname -m)"
            exit 1
            ;;
    esac

    echo "Detected platform: $OS-$ARCH"
}

# Build download URL
get_download_info() {
    local os="$1"
    local arch="$2"

    case "$os-$arch" in
        darwin-arm64)
            FILENAME="v-darwin-arm64.zip"
            ;;
        darwin-amd64)
            FILENAME="v-darwin-amd64.zip"
            ;;
        linux-arm64)
            FILENAME="v-linux-arm64.zip"
            ;;
        linux-amd64)
            FILENAME="v-linux-amd64.zip"
            ;;
        android-arm64)
            FILENAME="v-android-arm64.zip"
            ;;
        android-amd64)
            FILENAME="v-android-amd64.zip"
            ;;
        *)
            echo "No package available for $os-$arch"
            exit 1
            ;;
    esac

    DOWNLOAD_URL="${REPO}/releases/download/${VERSION}/${FILENAME}"
}

# Download file
download_file() {
    local url="$1"
    local output_file="$2"

    if command -v curl &> /dev/null; then
        curl -L -o "$output_file" "$url"
    elif command -v wget &> /dev/null; then
        wget -O "$output_file" "$url"
    else
        echo "Error: curl or wget is required for downloading"
        exit 1
    fi

    # Check download success
    if [ ! -f "$output_file" ] || [ ! -s "$output_file" ]; then
        echo "Error: Download failed or file is empty"
        exit 1
    fi
}

# Check if unzip is available
check_unzip() {
    if ! command -v unzip &> /dev/null; then
        echo "Error: unzip is required to extract the package"
        echo "Please install unzip first:"
        echo "  - Debian/Ubuntu: sudo apt install unzip"
        echo "  - macOS: brew install unzip"
        echo "  - Termux: pkg install unzip"
        exit 1
    fi
}

# Get SHA256 hash
get_sha256_hash() {
    local sha256_url="$1"
    local temp_sha_file="$TEMP_DIR/$(basename "$sha256_url")"

    download_file "$sha256_url" "$temp_sha_file"

    # Extract hash from SHA256 file
    if grep -q " " "$temp_sha_file"; then
        cut -d ' ' -f 1 "$temp_sha_file"
    else
        cat "$temp_sha_file"
    fi
}

# Verify SHA256
verify_sha256() {
    local file="$1"
    local expected_sha="$2"

    if ! command -v shasum &> /dev/null && ! command -v sha256sum &> /dev/null; then
        echo "Warning: shasum or sha256sum not found, skipping verification"
        return 0
    fi

    local actual_sha=""
    if command -v shasum &> /dev/null; then
        actual_sha=$(shasum -a 256 "$file" | cut -d ' ' -f1)
    elif command -v sha256sum &> /dev/null; then
        actual_sha=$(sha256sum "$file" | cut -d ' ' -f1)
    fi

    if [ "$actual_sha" != "$expected_sha" ]; then
        echo "SHA256 verification failed!"
        echo "Expected: $expected_sha"
        echo "Actual:   $actual_sha"
        return 1
    fi

    echo "SHA256 verification passed"
    return 0
}

# Ensure directory exists and return whether sudo is needed
ensure_dir_exists() {
    local dir="$1"
    local need_sudo=0

    if [ ! -d "$dir" ]; then
        echo "Creating directory: $dir"
        if mkdir -p "$dir" 2>/dev/null; then
            echo "Directory created successfully"
            need_sudo=0
        else
            echo "Need sudo to create directory"
            if sudo mkdir -p "$dir"; then
                need_sudo=1
            else
                echo "Error: Cannot create directory: $dir"
                exit 1
            fi
        fi
    elif [ ! -w "$dir" ]; then
        echo "Directory $dir exists but is not writable"
        need_sudo=1
    else
        need_sudo=0
    fi

    return $need_sudo
}

# Install binary
install_binary() {
    local zip_file="$1"
    local install_dir="$2"

    echo "Extracting files..."
    unzip -q "$zip_file" -d "$TEMP_DIR"

    local binary_path="$TEMP_DIR/$BINARY_NAME"

    if [ ! -f "$binary_path" ]; then
        echo "Error: Cannot find $BINARY_NAME in archive"
        exit 1
    fi

    echo "Installing to $install_dir"
    chmod +x "$binary_path"

    if ensure_dir_exists "$install_dir"; then
        mv "$binary_path" "$install_dir/"
        echo "Binary moved successfully"
    else
        echo "Need sudo to write to $install_dir"
        sudo mv "$binary_path" "$install_dir/"
        echo "Binary moved successfully (using sudo)"
    fi

    # Verify installation
    if command -v "$BINARY_NAME" &> /dev/null; then
        echo "Installation successful!"
        echo "$BINARY_NAME version: $($BINARY_NAME --version 2>/dev/null || echo 'installed')"
    else
        echo "Note: $BINARY_NAME may not be in your PATH"
        echo "Installed at: $install_dir"
        echo "Please ensure $install_dir is in your PATH environment variable"

        # Suggest adding to PATH
        local shell_rc=""
        if [ -f "$HOME/.bashrc" ]; then
            shell_rc="$HOME/.bashrc"
        elif [ -f "$HOME/.zshrc" ]; then
            shell_rc="$HOME/.zshrc"
        elif [ -f "$HOME/.profile" ]; then
            shell_rc="$HOME/.profile"
        fi

        if [ -n "$shell_rc" ]; then
            echo ""
            echo "To add to PATH, run:"
            echo "  echo 'export PATH=\"$install_dir:\$PATH\"' >> $shell_rc"
            echo "  source $shell_rc"
        fi
    fi
}

# Main installation flow
main() {
    echo "Installing $BINARY_NAME $VERSION"

    # Check for unzip
    check_unzip

    # Determine install directory
    INSTALL_DIR=$(determine_install_dir)
    echo "Installation directory: $INSTALL_DIR"

    # Detect platform
    detect_platform

    # Get download info
    get_download_info "$OS" "$ARCH"

    echo "Download URL: $DOWNLOAD_URL"

    # Download file
    local zip_file="$TEMP_DIR/$FILENAME"
    download_file "$DOWNLOAD_URL" "$zip_file"

    # Get and verify SHA256
    local sha256_url="${DOWNLOAD_URL}.sha256"
    echo "Fetching SHA256 checksum: $sha256_url"

    local expected_sha=""
    if expected_sha=$(get_sha256_hash "$sha256_url"); then
        echo "Verifying file integrity..."
        if ! verify_sha256 "$zip_file" "$expected_sha"; then
            echo "Error: SHA256 verification failed, aborting installation"
            exit 1
        fi
    else
        echo "Warning: Cannot fetch SHA256 checksum, skipping verification"
    fi

    # Install
    install_binary "$zip_file" "$INSTALL_DIR"
}

# Run main function
main "$@"
