import { specification } from '@jterrazz/test';
import { execSync } from 'node:child_process';
import { existsSync, readdirSync, statSync } from 'node:fs';
import { resolve } from 'node:path';
import { afterAll } from 'vitest';

const REPO_ROOT = resolve(import.meta.dirname, '../..');
const STATION_BIN = resolve(REPO_ROOT, '.artifacts/go/station-test');
const SOURCES = resolve(REPO_ROOT, 'src');

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
// an unstubbed tool is "not found" alike on a Mac and on the Linux CI runner; the
// machine kind is pinned to host unless a document says otherwise; GitHub is a closed
// port unless a document serves its stub, and so is Tailscale's API.
export const { cleanup, cli } = await specification.cli(STATION_BIN, {
    defaults: {
        COLUMNS: '80',
        HOME: '$WORKDIR',
        NO_COLOR: '1',
        PATH: '$WORKDIR/bin',
        STATION_GITHUB_API_URL: 'http://127.0.0.1:9',
        STATION_MACHINE_KIND: 'host',
        STATION_SYSTEM_DIRS: '',
        STATION_TAILSCALE_API_URL: 'http://127.0.0.1:9',
        TZ: 'UTC',
    },
    serve: {
        github: {
            command: 'node specs/cli/__setup__/github-api.ts',
            env: 'STATION_GITHUB_API_URL',
            ready: /listening on port (?<port>\d+)/u,
            url: (port) => `http://127.0.0.1:${port}`,
        },
        tailscale: {
            command: 'node specs/cli/__setup__/tailscale-api.ts',
            env: 'STATION_TAILSCALE_API_URL',
            ready: /listening on port (?<port>\d+)/u,
            url: (port) => `http://127.0.0.1:${port}`,
        },
    },
});

afterAll(cleanup);
