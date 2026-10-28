#!/bin/sh
# Installs the latest slb release on Linux, macOS or WSL.
#
#   curl -fsSL https://raw.githubusercontent.com/te-o-o-o/dg-statuslines/main/install.sh | sh
#
# The binary goes to ~/.local/bin. Running the script again updates slb in place,
# so the path Claude Code runs never changes.
set -eu

repo="te-o-o-o/dg-statuslines"
dir="$HOME/.local/bin"

case "$(uname -s)" in
  Linux) os=linux ;;
  Darwin) os=darwin ;;
  *) echo "slb: unsupported system $(uname -s). On Windows, use install.ps1." >&2; exit 1 ;;
esac
case "$(uname -m)" in
  x86_64 | amd64) arch=amd64 ;;
  arm64 | aarch64) arch=arm64 ;;
  *) echo "slb: unsupported architecture $(uname -m)." >&2; exit 1 ;;
esac

file="slb-$os-$arch"
base="https://github.com/$repo/releases/latest/download"
tmp=$(mktemp)
trap 'rm -f "$tmp"' EXIT

echo "Downloading $file..."
curl -fsSL "$base/$file" -o "$tmp"

# Refuse the binary unless it matches the checksum published with the release.
want=$(curl -fsSL "$base/checksums.txt" | awk -v f="$file" '$2 == f { print $1 }')
if command -v sha256sum >/dev/null 2>&1; then
  have=$(sha256sum "$tmp" | cut -d' ' -f1)
else
  have=$(shasum -a 256 "$tmp" | cut -d' ' -f1)
fi
if [ -z "$want" ] || [ "$want" != "$have" ]; then
  echo "slb: checksum mismatch, nothing installed." >&2
  exit 1
fi

mkdir -p "$dir"
chmod +x "$tmp"
mv "$tmp" "$dir/slb"
echo "Installed $("$dir/slb" version) in $dir/slb"

case ":$PATH:" in
  *":$dir:"*) echo "Run: slb" ;;
  *) echo "Add $dir to your PATH, then run: slb" ;;
esac
