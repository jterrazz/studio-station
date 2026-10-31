import { test } from '@jterrazz/test/config';
import { defineConfig } from '@jterrazz/typescript/config';

export default defineConfig({
    kind: 'cli',
    platform: 'node',
    presets: [test()],
    typescript: {
        knip: {
            entry: {
                'specs/cli/cli.specification.ts':
                    'the runner every `<case>.spec.yaml` document is bound to by the literate plugin; no TypeScript spec imports it',
                'specs/cli/__setup__/github-api.ts':
                    'the GitHub API stub a document serves (`serve: [github]`); the runner spawns it with node, nothing imports it',
            },
            ignoreBinaries: {
                go: 'the CLI specification builds the Go binary it drives; the toolchain is a machine requirement, never an npm dependency',
            },
        },
        roots: {
            scripts: 'the curl-piped installer a fresh machine runs before `station` exists on it',
        },
    },
});
