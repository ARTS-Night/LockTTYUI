#!/usr/bin/env bash
# LockTTYUI RPM release installer for AlmaLinux 9 x86_64.
# Fetches release metadata, downloads the matching official RPM and checks
# its GitHub-asset SHA-256 *before* passing it to DNF. No remote shell execution.
set -Eeuo pipefail

usage() {
  cat <<'HELP'
LockTTYUI RPM manager (AlmaLinux 9 x86_64)
Usage:
  bash install.sh install [build-N]  Install/update the latest published build, or a pinned build
  bash install.sh remove             Remove RPM and restore stock getty configuration
  bash install.sh status             Show installed package and TTY1 unit states
  bash install.sh --help

Run from SSH or TTY2, not TTY1. The installer NEVER restarts a logged-in TTY1.
After initial install, activate the dashboard on an idle TTY1 from SSH/TTY2:
  sudo systemctl stop getty@tty1.service
  sudo systemctl start lockttyui-tty1.service

The release RPM is currently unsigned. SHA-256 is checked against GitHub's
official release-asset digest, but this does NOT provide an independent signature.
HELP
}

die(){ printf 'Error: %s\n' "$*" >&2; exit 1; }

action="${1:---help}"
case "$action" in
  --help|-h|help) usage; exit 0 ;;
  install|remove|status) ;;
  *) usage >&2; die "Unknown action: $action" ;;
esac

if [[ "$(tty 2>/dev/null || true)" == "/dev/tty1" ]]; then
  die "Please use SSH or TTY2. Never run this from TTY1."
fi
if [[ ! -r /etc/os-release ]]; then die "Cannot detect distribution"; fi
# shellcheck disable=SC1091
source /etc/os-release
if [[ "${ID:-}" != "almalinux" || "${VERSION_ID:-}" != 9* || "$(uname -m)" != "x86_64" ]]; then
  die "Only AlmaLinux 9 x86_64 is supported by this RPM."
fi

if [[ "$action" == "status" ]]; then
  rpm -q lockttyui || true
  if command -v systemctl >/dev/null 2>&1; then
    systemctl show getty@tty1.service lockttyui-tty1.service lockttyui-tty1-handoff.service \
      -p Id -p ActiveState -p SubState -p Result --no-pager || true
  fi
  exit 0
fi

command -v sudo >/dev/null || die "sudo is required"
sudo -v
if [[ "$action" == "remove" ]]; then
  sudo dnf remove -y lockttyui
  echo "Removed. The RPM restores the stock TTY1 getty configuration."
  exit 0
fi

[[ $# -le 2 ]] || die "Usage: install [build-N]"
requested="${2:-}"
if [[ -n "$requested" && ! "$requested" =~ ^build-[0-9]+$ ]]; then
  die "Release must look like build-13"
fi
for dep in curl python3 sha256sum dnf rpm; do
  command -v "$dep" >/dev/null || die "Missing prerequisite: $dep"
done

echo "Checking official GitHub Releases..."
api="https://api.github.com/repos/ARTS-Night/LockTTYUI/releases"
if [[ -n "$requested" ]]; then
  api="$api/tags/$requested"
else
  api="$api?per_page=100"
fi
metadata="$(curl --fail --silent --show-error --location --retry 3 \
  --connect-timeout 10 --max-time 40 --header 'Accept: application/vnd.github+json' "$api")"

# Parse with Python's standard library, not grep/sed over JSON. The official
# repo, asset basename, URL path, build number and digest are all constrained.
# With no tag specified, choose the highest numeric build-N (not list order).
selection="$(printf '%s' "$metadata" | python3 -c '
import json, re, sys
requested=sys.argv[1]
data=json.load(sys.stdin)
items=data if isinstance(data,list) else [data]
items=[x for x in items if isinstance(x,dict) and not x.get("draft",True) and re.fullmatch(r"build-[0-9]+",str(x.get("tag_name","")))]
if requested:
    items=[x for x in items if x["tag_name"]==requested]
else:
    items.sort(key=lambda x:int(x["tag_name"].split("-")[1]),reverse=True)
for item in items:
    tag=item["tag_name"]
    number=tag.split("-")[1]
    for a in item.get("assets",[]):
        name=a.get("name","")
        url=a.get("browser_download_url","")
        digest=a.get("digest","")
        if not re.fullmatch(r"lockttyui-[0-9]+\.[0-9]+\.[0-9]+-"+re.escape(number)+r"\.el9\.x86_64\.rpm",name):
            continue
        if url != "https://github.com/ARTS-Night/LockTTYUI/releases/download/"+tag+"/"+name:
            continue
        if not re.fullmatch(r"sha256:[0-9a-fA-F]{64}",str(digest)):
            continue
        print(tag,name,url,digest[7:].lower(),sep="\t")
        sys.exit(0)
print("No supported release asset with SHA256 digest found",file=sys.stderr)
sys.exit(1)
' "$requested")"

IFS=$'\t' read -r tag rpm_name rpm_url digest <<<"$selection"
[[ -n "$tag" && -n "$rpm_name" && -n "$rpm_url" && "${#digest}" -eq 64 ]] || die "Invalid release metadata"

workdir="$(mktemp -d)"
trap 'rm -rf "$workdir"' EXIT
echo "Selected $tag ($rpm_name)"
curl --fail --silent --show-error --location --retry 3 \
  --connect-timeout 10 --max-time 120 -o "$workdir/$rpm_name" "$rpm_url"
printf '%s  %s\n' "$digest" "$workdir/$rpm_name" | sha256sum --check --status \
  || die "SHA-256 mismatch; nothing installed"
echo "SHA-256 verified."

# A package upgrade preserves any active TTY1 login. It does not implicitly
# restart getty, switch VTs, touch SELinux policy or change system-wide printk.
sudo dnf install -y --nogpgcheck "$workdir/$rpm_name"
rpm -q lockttyui
echo "Done. Active TTY1 sessions were not restarted."
echo "If TTY1 was already running, log in/out to launch the new binary."
echo "If the dashboard has never been activated, see: bash install.sh --help"
