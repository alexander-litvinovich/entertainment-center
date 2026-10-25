# Releasing hdmi-cec

One version covers the firmware and the Windows host: [`hdmi-cec/VERSION`](../VERSION). The
firmware reports `cec-host-<VERSION>-<source hash>`; the host EXE reports `<VERSION>`.

1. Bump `hdmi-cec/VERSION` (`X.Y.Z`). Releases sort by these numbers, and installed services
   only move to a higher version.
2. If the USB protocol changed, update the minimum firmware check in
   [`host/internal/device/serial.go`](../host/internal/device/serial.go) (`checkVersion`).
3. Run the tests:

   ```sh
   hdmi-cec/.venv/bin/python hdmi-cec/tests/run.py
   (cd hdmi-cec/host && go test ./... && GOOS=windows go vet ./...)
   ```

4. Commit, tag and push the branch and the tag separately (GitHub can drop the tag event when
   both go in one push, and then no workflow starts):

   ```sh
   git commit -am "hdmi-cec X.Y.Z"
   git tag hdmi-cec-vX.Y.Z
   git push origin master
   git push origin hdmi-cec-vX.Y.Z
   ```

   If `gh run list` shows no run a minute later, re-push the tag:
   `git push origin :refs/tags/hdmi-cec-vX.Y.Z && git push origin hdmi-cec-vX.Y.Z`.

The [release workflow](../../.github/workflows/release-hdmi-cec.yml) checks that the tag
matches `VERSION`, runs the tests, builds `hdmi-cec.exe` (Linux runner) and the firmware
(macOS runner), and publishes a GitHub release with `hdmi-cec.exe`, `pico-cec.uf2`,
`pico-cec.elf`, `manifest.json` and `SHA256SUMS`.

Installed Windows services pick the release up within 24 hours, or immediately with
`hdmi-cec.exe update`. Boards are flashed by hand:
`python3 .agents/skills/_shared/cec.py download` and then `flash --image …`
([FLASHING.md](FLASHING.md)).

A failed run publishes nothing. Fix the problem, then move the tag:
`git tag -f hdmi-cec-vX.Y.Z && git push -f origin hdmi-cec-vX.Y.Z`. Once a release is
published, ship fixes as a new version instead: services may already have installed it.
