#!/bin/sh
set -e

# gemini-notes-export installer (fresh install and upgrade).
# Usage: curl -fsSL https://raw.githubusercontent.com/chrisedwards/gemini-notes-export/main/install.sh | sh
# Custom dir: curl -fsSL ... | INSTALL_DIR=/path/to/dir sh
#
# Installs gemini-notes-export and the bundled gog into INSTALL_DIR. The bundled
# gog is skipped when a gog of the same or newer version is already on PATH or
# in INSTALL_DIR, so a newer system gog is never replaced or shadowed.

REPO="chrisedwards/gemini-notes-export"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"

detect_os() {
    case "$(uname -s)" in
        Darwin*) echo "darwin" ;;
        Linux*)  echo "linux" ;;
        MINGW*|MSYS*|CYGWIN*) echo "windows" ;;
        *) echo "unsupported" ;;
    esac
}

detect_arch() {
    case "$(uname -m)" in
        x86_64|amd64) echo "amd64" ;;
        arm64|aarch64) echo "arm64" ;;
        *) echo "unsupported" ;;
    esac
}

get_latest_version() {
    curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" |
        grep '"tag_name"' |
        sed -E 's/.*"([^"]+)".*/\1/'
}

# Prints "X.Y.Z" from a binary's version output, or nothing.
version_of() {
    "$@" 2>/dev/null | head -1 | grep -Eo '[0-9]+\.[0-9]+\.[0-9]+' | head -1
}

# Succeeds when version $1 >= version $2.
version_ge() {
    [ -n "$1" ] && [ -n "$2" ] || return 1
    awk -v a="$1" -v b="$2" 'BEGIN {
        split(a, x, "."); split(b, y, ".")
        for (i = 1; i <= 3; i++) { if (x[i] + 0 != y[i] + 0) exit !(x[i] + 0 > y[i] + 0) }
        exit 0
    }'
}

main() {
    OS=$(detect_os)
    ARCH=$(detect_arch)
    if [ "$OS" = "unsupported" ] || [ "$ARCH" = "unsupported" ]; then
        echo "Error: Unsupported platform: $(uname -s) $(uname -m)"
        echo "Download manually from https://github.com/${REPO}/releases"
        exit 1
    fi
    echo "Detecting platform... ${OS}-${ARCH}"

    CURRENT_VERSION=""
    if command -v gemini-notes-export >/dev/null 2>&1; then
        CURRENT_VERSION=$(version_of gemini-notes-export version)
    fi

    VERSION=$(get_latest_version)
    if [ -z "$VERSION" ]; then
        echo "Error: Could not determine latest version"
        exit 1
    fi
    echo "Latest version: ${VERSION}"

    if [ "$OS" = "windows" ]; then
        EXT=".exe"
        ARCHIVE="gemini-notes-export-${VERSION}-${OS}-${ARCH}.zip"
    else
        EXT=""
        ARCHIVE="gemini-notes-export-${VERSION}-${OS}-${ARCH}.tar.gz"
    fi
    URL="https://github.com/${REPO}/releases/download/${VERSION}/${ARCHIVE}"

    echo "Downloading ${ARCHIVE}..."
    TMP=$(mktemp -d)
    trap 'rm -rf "$TMP"' EXIT
    curl -fsSL "$URL" -o "${TMP}/${ARCHIVE}"

    echo "Extracting..."
    cd "$TMP"
    if [ "$OS" = "windows" ]; then
        unzip -q "$ARCHIVE"
    else
        tar -xzf "$ARCHIVE"
    fi

    echo "Installing to ${INSTALL_DIR}..."
    mkdir -p "$INSTALL_DIR"
    mv "gemini-notes-export${EXT}" "${INSTALL_DIR}/"

    BUNDLED_GOG=$(version_of "./gog${EXT}" --version)
    EXISTING_GOG=""
    if [ -x "${INSTALL_DIR}/gog${EXT}" ]; then
        EXISTING_GOG=$(version_of "${INSTALL_DIR}/gog${EXT}" --version)
    elif command -v gog >/dev/null 2>&1; then
        EXISTING_GOG=$(version_of gog --version)
    fi
    if version_ge "$EXISTING_GOG" "$BUNDLED_GOG"; then
        echo "Keeping existing gog ${EXISTING_GOG} (bundled is ${BUNDLED_GOG:-unknown})"
    else
        mv "gog${EXT}" "${INSTALL_DIR}/"
        echo "Installed gog ${BUNDLED_GOG:-(bundled)} to ${INSTALL_DIR}"
    fi

    echo ""
    if [ -n "$CURRENT_VERSION" ]; then
        echo "Upgraded gemini-notes-export: ${CURRENT_VERSION} -> ${VERSION}"
        return
    fi

    echo "Installed gemini-notes-export ${VERSION}"
    echo ""
    case ":$PATH:" in
        *":$INSTALL_DIR:"*) ;;
        *)
            echo "Add ${INSTALL_DIR} to your PATH, for example:"
            echo ""
            echo "  echo 'export PATH=\"${INSTALL_DIR}:\$PATH\"' >> ~/.zshrc"
            echo ""
            ;;
    esac
    echo "Next steps:"
    echo "  1. Set up config:            gemini-notes-export init"
    echo "  2. Authorize Google (gog):   see https://github.com/${REPO}#google-auth-setup"
    echo "  3. Export your notes:        gemini-notes-export sync --since YYYY-MM-DD"
}

main
