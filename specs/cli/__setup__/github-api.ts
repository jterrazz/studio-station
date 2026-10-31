import { readFile } from 'node:fs/promises';
import { createServer } from 'node:http';
import type { IncomingMessage, ServerResponse } from 'node:http';
import { join } from 'node:path';

// invariant: the GitHub API a run talks to, answered from the run's own files: a GET of
// /repos/<owner>/<repo>/contents/<folder> answers answers/github/<the same path>.json of
// the document's working directory, and anything without an answer file is a 404.
const root = join(process.env.TEST_WORKDIR ?? '.', 'answers', 'github');

async function answer(request: IncomingMessage, response: ServerResponse): Promise<void> {
    const path = new URL(request.url ?? '/', 'http://stub').pathname.replace(/\/+$/u, '');
    try {
        const body = await readFile(join(root, `${path}.json`));
        response.writeHead(200, { 'content-type': 'application/json' });
        response.end(body);
    } catch {
        response.writeHead(404);
        response.end();
    }
}

const server = createServer((request, response) => {
    void answer(request, response);
});

server.listen(0, '127.0.0.1', () => {
    const address = server.address();
    const port = typeof address === 'object' && address !== null ? address.port : 0;
    process.stdout.write(`listening on port ${port}\n`);
});
