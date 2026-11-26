#!/bin/bash
set -Eeuo pipefail
export DEBIAN_FRONTEND=noninteractive
apt-get update
apt-get install -y --no-install-recommends systemd systemd-sysv util-linux login
unitdir=/lib/systemd/system
if [ ! -f "$unitdir/getty@.service" ]; then unitdir=/usr/lib/systemd/system; fi
stock=$(sha256sum "$unitdir/getty@.service")
set -- /packages/*.deb
[ "$#" = 1 ]
pkg="$1"
for script in /packaging/debian/{postinst,prerm,postrm}; do sh -n "$script"; done
apt-get install -y --no-install-recommends "$pkg"
dpkg-query -W lockttyui
dpkg -V lockttyui
test -x /usr/bin/lockttyui
test -x /usr/libexec/lockttyui-tty1-route
getent passwd nobody >/dev/null
! grep -q '^Group=' "$unitdir/lockttyui-tty1.service"
test -L "$unitdir/getty.target.wants/lockttyui-tty1.service"
! grep -q '^ExecStart=' "$unitdir/getty@tty1.service.d/50-lockttyui.conf"
test ! -e "$unitdir/getty@tty2.service.d/50-lockttyui.conf"
test ! -e /etc/systemd/system/getty@tty1.service.d/50-lockttyui.conf
systemd-analyze --man=no verify getty@tty1.service lockttyui-tty1.service lockttyui-tty1-handoff.service lockttyui-tty1-return.service lockttyui-tty1-fallback.service
/usr/bin/lockttyui --glyph-test
apt-get install -y --reinstall --no-install-recommends "$pkg"
dpkg -V lockttyui
apt-get remove -y lockttyui
for service in lockttyui-preview lockttyui-tty1 lockttyui-tty1-handoff lockttyui-tty1-return lockttyui-tty1-fallback; do
  test ! -e "$unitdir/$service.service"
done
test ! -e /usr/bin/lockttyui
test ! -e /usr/libexec/lockttyui-tty1-route
test ! -e "$unitdir/getty@tty1.service.d/50-lockttyui.conf"
test ! -e "$unitdir/getty.target.wants/lockttyui-tty1.service"
apt-get purge -y lockttyui
test "$(sha256sum "$unitdir/getty@.service")" = "$stock"
