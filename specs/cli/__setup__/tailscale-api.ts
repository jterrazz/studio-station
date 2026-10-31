import { appendFile, readFile } from 'node:fs/promises';
import { createServer } from 'node:http';
import type { IncomingMessage, ServerResponse } from 'node:http';
import { join } from 'node:path';

// invariant: the Tailscale API a run talks to, answered from the run's own files and
// recorded in its working directory: every request appends `<METHOD> <path>` and its
// body to .api; the OAuth token, the key and the device removal answer
// answers/tailscale-api/{token,keys,device}.json, with the status in the matching
// .status file (default 200), and anything without an answer file is a 404.
const workdir = process.env.TEST_WORKDIR ?? '.';
const root = join(workdir, 'answers', 'tailscale-api');

function answerName(method: string, path: string): string {
    if (path === '/api/v2/oauth/token') {
        return 'token';
    }
    if (path === '/api/v2/tailnet/-/keys') {
        return 'keys';
    }
    if (method === 'DELETE' && path.startsWith('/api/v2/device/')) {
        return 'device';
    }
    return 'none';
}

async function read(file: string): Promise<string | undefined> {
    try {
        return await readFile(file, 'utf8');
    } catch {
        return undefined;
    }
}

async function answer(request: IncomingMessage, response: ServerResponse): Promise<void> {
    let body = '';
    for await (const chunk of request) {
        body += String(chunk);
    }
    const method = request.method ?? 'GET';
    const path = new URL(request.url ?? '/', 'http://stub').pathname;
    await appendFile(join(workdir, '.api'), `${method} ${path}\n${body}\n`);
    const name = answerName(method, path);
    const reply = await read(join(root, `${name}.json`));
    if (reply === undefined) {
        response.writeHead(404);
        response.end();
        return;
    }
    const statusFile = await read(join(root, `${name}.status`));
    const status = Number(statusFile?.trim() ?? '200');
    response.writeHead(status, { 'content-type': 'application/json' });
    response.end(reply);
}

const server = createServer((request, response) => {
    void answer(request, response);
});

server.listen(0, '127.0.0.1', () => {
    const address = server.address();
    const port = typeof address === 'object' && address !== null ? address.port : 0;
    process.stdout.write(`listening on port ${port}\n`);
});
