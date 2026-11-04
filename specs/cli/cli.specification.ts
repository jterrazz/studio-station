import { specification } from '@jterrazz/test';
import { execSync } from 'node:child_process';
import { existsSync, readdirSync, renameSync, statSync } from 'node:fs';
import { resolve } from 'node:path';
import { afterAll } from 'vitest';

const REPO_ROOT = resolve(import.meta.dirname, '../..');
const STATION_BIN = resolve(REPO_ROOT, '.artifacts/go/station-test');
const SOURCES = resolve(REPO_ROOT, 'src');
const VM_SSHD = resolve(import.meta.dirname, '__fixtures__/vm-sshd');
const WORLDS = resolve(REPO_ROOT, 'specs/usecases/worlds');

// hazard: a stale binary silently tests old behaviour, so the suite rebuilds when any
// file under `src/` is newer than the binary: the embedded dotfiles and recipes are
// build inputs as much as the Go sources are.
function binaryIsStale(): boolean {
    if (!existsSync(STATION_BIN)) {
        return true;
    }
    const binModifiedMs = statSync(STATION_BIN).mtimeMs;
    for (const entry of readdirSync(SOURCES, { recursive: true, withFileTypes: true })) {
        if (
            entry.isFile() &&
            statSync(resolve(entry.parentPath, entry.name)).mtimeMs > binModifiedMs
        ) {
            return true;
        }
    }
    return false;
}

// invariant: `make test` builds the binary before the runner starts; a direct run that
// finds it stale builds it beside and renames it into place, so a worker never runs a
// binary another worker is still writing.
if (binaryIsStale()) {
    const version = execSync('git describe --tags --always --dirty', { cwd: REPO_ROOT })
        .toString()
        .trim();
    const building = `${STATION_BIN}.${process.pid}`;
    execSync(
        `go build -tags world -ldflags "-X github.com/jterrazz/studio-station/src/wire.version=${version}" -o ${building} ./src/cmd/station`,
        { cwd: REPO_ROOT, stdio: 'inherit' },
    );
    renameSync(building, STATION_BIN);
}

// invariant: no run reaches the real machine. HOME is the run's own directory, PATH
// holds only the fixture's stubs and STATION_SYSTEM_DIRS adds no system directory, so
// an unstubbed tool is "not found" alike on a Mac and on the Linux CI runner;
// STATION_SYSTEM_ROOT reads /Applications, /opt/homebrew, /Library and /usr/local under
// the run's own `root/`, so the owner's apps are neither listed nor sized nor run;
// STATION_POWER_STUB points Power at the run's own `power/` (absent: no Power here), so no
// run takes a real assertion or touches the clamshell; the
// machine kind is pinned to host unless a document says otherwise; GitHub is a closed
// port unless a document serves its stub; a VM's sshd is the served stub vm-sshd.
// value: one env set per world of specs/usecases/worlds/, named after it: a
// use-case document that writes `env: two-macs` runs the spec build over a copy of
// that world, `world.json` in its own directory, read and written by every run.
const worlds = Object.fromEntries(
    readdirSync(WORLDS)
        .filter((name) => name.endsWith('.json'))
        .map((name) => [
            name.replace(/\.json$/u, ''),
            { STATION_WORLD: '$WORKDIR/world.json', STATION_WORLD_SEED: resolve(WORLDS, name) },
        ]),
);

export const { cleanup, cli } = await specification.cli(STATION_BIN, {
    defaults: {
        COLUMNS: '80',
        HOME: '$WORKDIR',
        NO_COLOR: '1',
        PATH: '$WORKDIR/bin',
        STATION_GITHUB_API_URL: 'http://127.0.0.1:9',
        STATION_MACHINE_KIND: 'host',
        STATION_POWER_STUB: '$WORKDIR/power',
        STATION_SYSTEM_DIRS: '',
        STATION_SYSTEM_ROOT: '$WORKDIR/root',
        TZ: 'UTC',
    },
    // value: a document that drives the TUI runs `terminal "$STATION"`, the binary on a
    // pseudo-terminal its fixture's driver opens, since a run is never given a TTY; one
    // that installs runs `$INSTALL_SH` from a local file through its one test seam.
    env: {
        ...worlds,
        daemon: { STATION: STATION_BIN },
        install: { INSTALL_SH: resolve(REPO_ROOT, 'scripts/install.sh'), STATION: STATION_BIN },
        terminal: { STATION: STATION_BIN, TERM: 'xterm-256color' },
    },
    serve: {
        github: {
            command: 'node specs/cli/__setup__/github-api.ts',
            env: 'STATION_GITHUB_API_URL',
            ready: /listening on port (?<port>\d+)/u,
            url: (port) => `http://127.0.0.1:${port}`,
        },
        sshd: {
            command: `STATION_TEST_BIN=${STATION_BIN} go run ${VM_SSHD}`,
            env: 'STATION_GUEST_SSH_PORT',
            ready: /listening on port (?<port>\d+)/u,
            url: String,
        },
    },
});

afterAll(cleanup);
