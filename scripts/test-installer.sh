#!/bin/sh
# Offline regression check for the website-hosted Unix installer. Exercises
# OS/architecture resolution, checksum verification and existing-file safety.
set -eu
root=$(mktemp -d "${TMPDIR:-/tmp}/dm-dot-installer-test.XXXXXXXX")
trap 'rm -rf "$root"' EXIT HUP INT TERM
mkdir -p "$root/fake-bin" "$root/assets" "$root/target"
cat > "$root/fake-bin/uname" <<'EOF'
#!/bin/sh
case "$1" in
  -s) printf '%s\n' "$FAKE_OS" ;;
  -m) printf '%s\n' "$FAKE_ARCH" ;;
  *) exit 2 ;;
esac
EOF
cat > "$root/fake-bin/curl" <<'EOF'
#!/bin/sh
out=''
url=''
while [ "$#" -gt 0 ]; do
  case "$1" in
    -o) out=$2; shift 2 ;;
    -w) shift 2 ;;
    *) url=$1; shift ;;
  esac
done
case "$url" in
  */SHA256SUMS) cp "$FAKE_ASSETS/SHA256SUMS" "$out" ;;
  */dm-dot_*) cp "$FAKE_ASSETS/${url##*/}" "$out" ;;
  *) echo "unexpected curl URL: $url" >&2; exit 2 ;;
esac
EOF
chmod +x "$root/fake-bin/uname" "$root/fake-bin/curl"
export PATH="$root/fake-bin:$PATH" FAKE_ASSETS="$root/assets" CLI_INSTALL_DIR="$root/target"

for platform in linux_amd64 linux_arm64 darwin_amd64 darwin_arm64; do
  case "$platform" in
    linux_amd64) FAKE_OS=Linux; FAKE_ARCH=x86_64 ;;
    linux_arm64) FAKE_OS=Linux; FAKE_ARCH=aarch64 ;;
    darwin_amd64) FAKE_OS=Darwin; FAKE_ARCH=x86_64 ;;
    darwin_arm64) FAKE_OS=Darwin; FAKE_ARCH=arm64 ;;
  esac
  export FAKE_OS FAKE_ARCH
  rm -f "$root/target/dm-dot" "$root/assets"/dm-dot_* "$root/assets/SHA256SUMS"
  asset="dm-dot_${platform}"
  printf '#!/bin/sh\nprintf "dm-dot v0.1.0\\n"\n' > "$root/assets/$asset"
  chmod +x "$root/assets/$asset"
  (cd "$root/assets" && sha256sum "$asset" > SHA256SUMS)
  sh docs/public/install.sh >/dev/null
  [ -x "$root/target/dm-dot" ] || { echo "not installed: $platform" >&2; exit 1; }
  [ "$("$root/target/dm-dot" --version)" = 'dm-dot v0.1.0' ] || exit 1
  if sh docs/public/install.sh >/dev/null 2>&1; then
    echo 'installer unexpectedly overwrote existing executable' >&2
    exit 1
  fi
  # Corrupt the expected checksum; an upgrade must leave the old file intact.
  printf '%064d  %s\n' 0 "$asset" > "$root/assets/SHA256SUMS"
  if sh docs/public/install.sh upgrade >/dev/null 2>&1; then
    echo 'installer accepted wrong checksum' >&2
    exit 1
  fi
  [ "$("$root/target/dm-dot" --version)" = 'dm-dot v0.1.0' ] || exit 1
done
printf 'Installer passed four OS/architecture selection and checksum tests.\n'
