#!/bin/bash

set -x

systemctl --user stop eco
systemctl --user disable eco

set -e

rm -r ~/.local/share/decred-eco
rm ~/.local/share/applications/ecogui.desktop
rm ~/.local/share/icons/ecogui.png
rm ~/.local/share/systemd/user/eco.service
rm ~/.decred-eco/eco.db
update-desktop-database ~/.local/share/applications/