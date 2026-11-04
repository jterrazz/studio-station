import { test } from '@jterrazz/test/config';
import { defineConfig } from '@jterrazz/typescript/config';

export default defineConfig({
    kind: 'cli',
    platform: 'node',
    presets: [test()],
    test: {
        projects: {
            cli: {
                literate: {
                    include: ['specs/cli/**/*.spec.yaml', 'specs/adapters/**/*.spec.yaml'],
                },
                reason: 'an adapter spec (`specs/adapters/<tech>/`) runs the binary as a cli document does, through the same runner',
            },
        },
    },
    typescript: {
        knip: {
            entry: {
                'specs/cli/cli.specification.ts':
                    'the runner every `<case>.spec.yaml` document is bound to by the literate plugin; no TypeScript spec imports it',
                'specs/cli/__setup__/github-api.ts':
                    'the GitHub API stub a document serves (`serve: [github]`); the runner spawns it with node, nothing imports it',
                'specs/cli/__setup__/tailscale-api.ts':
                    'the Tailscale API stub a document serves (`serve: tailscale`); the runner spawns it with node, nothing imports it',
            },
            ignoreBinaries: {
                go: 'the CLI specification builds the Go binary it drives; the toolchain is a machine requirement, never an npm dependency',
            },
        },
        oxfmt: {
            ignore: {
                'TODO.md': 'copied verbatim from the design',
            },
        },
        roots: {
            scripts: 'the curl-piped installer a fresh machine runs before `station` exists on it',
        },
    },
});
