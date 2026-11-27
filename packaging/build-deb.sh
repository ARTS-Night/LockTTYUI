#!/bin/bash
set -Eeuo pipefail
# Ensure package files and the control directory have conventional permissions.
umask 022
# Run from repository root with a tested static Linux amd64 binary.
[[ $# = 4 ]] || { echo "usage: build-deb.sh 12|13 BUILD BINARY OUTPUT_DIR" >&2; exit 2; }
case "$1" in
  12) units=/lib/systemd/system ;;
  13) units=/usr/lib/systemd/system ;;
  *) exit 2 ;;
esac
[[ "$2" =~ ^[1-9][0-9]*$ ]] || exit 2
version=$(awk '/^Version:/{print $2;exit}' packaging/lockttyui.spec)
[[ "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || exit 2
version="$version-$2+deb$1"
root=$(mktemp -d)
chmod 0755 "$root"
trap 'rm -rf "$root"' EXIT
mkdir -p "$root/DEBIAN" "$4"
cat > "$root/DEBIAN/control" <<EOF
Package: lockttyui
Version: $version
Architecture: amd64
Maintainer: LockTTYUI project
Section: admin
Priority: optional
Depends: systemd (>= 252), systemd-sysv, util-linux, login
Description: Read-only tty1 system telemetry dashboard
 A pre-login display with stock systemd getty handoff.
 TTY2+, SSH, PAM and distro authentication policies are untouched.
EOF
for action in postinst prerm postrm; do
  install -m 0755 "packaging/debian/$action" "$root/DEBIAN/$action"
done
install -D -m 0755 "$3" "$root/usr/bin/lockttyui"
install -D -m 0755 packaging/lockttyui-tty1-route "$root/usr/libexec/lockttyui-tty1-route"
for service in lockttyui-preview lockttyui-tty1 lockttyui-tty1-handoff lockttyui-tty1-return lockttyui-tty1-fallback; do
  install -D -m 0644 "packaging/$service.service" "$root$units/$service.service"
done
install -D -m 0644 packaging/50-lockttyui-tty1.conf "$root$units/getty@tty1.service.d/50-lockttyui.conf"
mkdir -p "$root$units/getty.target.wants"
ln -s ../lockttyui-tty1.service "$root$units/getty.target.wants/lockttyui-tty1.service"
install -D -m 0644 README.md "$root/usr/share/doc/lockttyui/README.md"
install -D -m 0644 packaging/OPERATIONS.md "$root/usr/share/doc/lockttyui/OPERATIONS.md"
(
  cd "$root"
  find . -path ./DEBIAN -prune -o -type f -printf '%P\0' | sort -z | xargs -0 md5sum > DEBIAN/md5sums
)
dpkg-deb --root-owner-group --build "$root" "$4/lockttyui_${version}_amd64.deb"
