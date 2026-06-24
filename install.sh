#!/usr/bin/env sh
#
# vocab installer.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/AndrewMeleka/vocab/main/install.sh | sh
#
# Options (environment variables):
#   VERSION    install a specific tag instead of the latest (e.g. VERSION=v0.1.0)
#   BIN_DIR    install location (default: /usr/local/bin, or ~/.local/bin if not writable)
#
# Examples:
#   curl -fsSL https://raw.githubusercontent.com/AndrewMeleka/vocab/main/install.sh | sh
#   curl -fsSL https://raw.githubusercontent.com/AndrewMeleka/vocab/main/install.sh | VERSION=v0.1.0 sh
#   curl -fsSL https://raw.githubusercontent.com/AndrewMeleka/vocab/main/install.sh | BIN_DIR=$HOME/bin sh

set -eu

REPO="AndrewMeleka/vocab"
APP="vocab"

# ---- pretty logging -------------------------------------------------------
info() { printf '\033[1;34m==>\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33mwarning:\033[0m %s\n' "$*" >&2; }
err()  { printf '\033[1;31merror:\033[0m %s\n' "$*" >&2; exit 1; }

# ---- prerequisites --------------------------------------------------------
have() { command -v "$1" >/dev/null 2>&1; }

if have curl; then
	dl() { curl -fsSL "$1"; }
	dl_to() { curl -fsSL "$1" -o "$2"; }
elif have wget; then
	dl() { wget -qO- "$1"; }
	dl_to() { wget -qO "$2" "$1"; }
else
	err "need either curl or wget installed"
fi

have tar || err "need 'tar' installed"

# ---- detect platform ------------------------------------------------------
os="$(uname -s)"
arch="$(uname -m)"

case "$os" in
	Linux)  os="linux" ;;
	Darwin) os="darwin" ;;
	*) err "unsupported OS: $os (Windows users: download the zip from https://github.com/$REPO/releases)" ;;
esac

case "$arch" in
	x86_64 | amd64) arch="amd64" ;;
	aarch64 | arm64) arch="arm64" ;;
	*) err "unsupported architecture: $arch" ;;
esac

# ---- resolve version ------------------------------------------------------
version="${VERSION:-}"
if [ -z "$version" ]; then
	info "Resolving latest release..."
	version="$(dl "https://api.github.com/repos/$REPO/releases/latest" \
		| grep '"tag_name":' \
		| head -n1 \
		| sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')"
	[ -n "$version" ] || err "could not determine latest version (set VERSION=vX.Y.Z to install a specific release)"
fi

# Archive names mirror .goreleaser.yaml: vocab_<os>_<arch>.tar.gz
asset="${APP}_${os}_${arch}.tar.gz"
url="https://github.com/$REPO/releases/download/$version/$asset"

# ---- download & extract ---------------------------------------------------
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT INT TERM

info "Downloading $APP $version ($os/$arch)..."
dl_to "$url" "$tmp/$asset" || err "download failed: $url"

# Best-effort checksum verification.
if dl_to "https://github.com/$REPO/releases/download/$version/checksums.txt" "$tmp/checksums.txt" 2>/dev/null; then
	if have sha256sum; then
		( cd "$tmp" && grep " $asset\$" checksums.txt | sha256sum -c - >/dev/null 2>&1 ) \
			&& info "Checksum verified." || warn "checksum verification skipped/failed"
	elif have shasum; then
		( cd "$tmp" && grep " $asset\$" checksums.txt | shasum -a 256 -c - >/dev/null 2>&1 ) \
			&& info "Checksum verified." || warn "checksum verification skipped/failed"
	fi
fi

info "Extracting..."
tar -xzf "$tmp/$asset" -C "$tmp" || err "failed to extract archive"
[ -f "$tmp/$APP" ] || err "binary '$APP' not found in archive"
chmod +x "$tmp/$APP"

# ---- choose install dir ---------------------------------------------------
bin_dir="${BIN_DIR:-/usr/local/bin}"

install_to() {
	# $1 = target dir
	if [ -d "$1" ] && [ -w "$1" ]; then
		mv "$tmp/$APP" "$1/$APP"
		return 0
	fi
	return 1
}

if install_to "$bin_dir"; then
	:
elif have sudo && [ "${BIN_DIR:-}" = "" ]; then
	info "Installing to $bin_dir (requires sudo)..."
	sudo mkdir -p "$bin_dir"
	sudo mv "$tmp/$APP" "$bin_dir/$APP"
else
	# Fall back to a user-writable location.
	bin_dir="$HOME/.local/bin"
	mkdir -p "$bin_dir"
	mv "$tmp/$APP" "$bin_dir/$APP"
fi

info "Installed $APP to $bin_dir/$APP"

# ---- PATH hint ------------------------------------------------------------
case ":$PATH:" in
	*":$bin_dir:"*) ;;
	*) warn "$bin_dir is not on your PATH. Add this to your shell profile:"
	   printf '\n    export PATH="%s:$PATH"\n\n' "$bin_dir" ;;
esac

info "Done! Run '$APP --help' to get started."
