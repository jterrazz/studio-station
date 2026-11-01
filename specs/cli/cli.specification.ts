import { specification } from '@jterrazz/test';
import { execSync } from 'node:child_process';
import { existsSync, readdirSync, statSync } from 'node:fs';
import { resolve } from 'node:path';
import { afterAll } from 'vitest';

const REPO_ROOT = resolve(import.meta.dirname, '../..');
const STATION_BIN = resolve(REPO_ROOT, '.artifacts/go/station-test');
const SOURCES = resolve(REPO_ROOT, 'src');
const VM_SSHD = resolve(import.meta.dirname, '__fixtures__/vm-sshd');

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

if (process.env.STATION_FORCE_REBUILD === '1' || binaryIsStale()) {
    const version = execSync('git describe --tags --always --dirty', { cwd: REPO_ROOT })
        .toString()
        .trim();
    execSync(
        `go build -ldflags "-X github.com/jterrazz/studio-station/src/app.version=${version}" -o ${STATION_BIN} ./src/cmd/station`,
        { cwd: REPO_ROOT, stdio: 'inherit' },
    );
}

// invariant: no run reaches the real machine. HOME is the run's own directory, PATH
// holds only the fixture's stubs and STATION_SYSTEM_DIRS adds no system directory, so
// an unstubbed tool is "not found" alike on a Mac and on the Linux CI runner;
// STATION_SYSTEM_ROOT reads /Applications, /opt/homebrew and /Library under the run's
// own `root/`, so the owner's apps are neither listed nor sized; the
// machine kind is pinned to host unless a document says otherwise; GitHub is a closed
// port unless a document serves its stub; a VM's sshd is the served stub vm-sshd.
export const { cleanup, cli } = await specification.cli(STATION_BIN, {
    defaults: {
        COLUMNS: '80',
        HOME: '$WORKDIR',
        NO_COLOR: '1',
        PATH: '$WORKDIR/bin',
        STATION_GITHUB_API_URL: 'http://127.0.0.1:9',
        STATION_MACHINE_KIND: 'host',
        STATION_SYSTEM_DIRS: '',
        STATION_SYSTEM_ROOT: '$WORKDIR/root',
        TZ: 'UTC',
    },
    // value: a document that drives the TUI runs `terminal "$STATION"`, the binary on a
    // pseudo-terminal its fixture's driver opens, since a run is never given a TTY; one
    // that installs runs `$INSTALL_SH` from a local file through its one test seam.
    env: {
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
