// Smoke test delle API: avvia il server su un DB temporaneo e verifica il flusso principale.
import { spawn } from 'node:child_process';
import { mkdtempSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import assert from 'node:assert/strict';

const PORT = 3123;
const BASE = `http://localhost:${PORT}/api`;
const dir = mkdtempSync(join(tmpdir(), 'f1test-'));

const server = spawn(process.execPath, ['--disable-warning=ExperimentalWarning', 'server.js'], {
  env: { ...process.env, PORT: String(PORT), DB_PATH: join(dir, 'test.db') },
  stdio: ['ignore', 'pipe', 'inherit'],
});
await new Promise((resolve) => server.stdout.once('data', resolve));

async function call(method, path, body) {
  const res = await fetch(BASE + path, {
    method,
    headers: { 'Content-Type': 'application/json' },
    body: body ? JSON.stringify(body) : undefined,
  });
  return { status: res.status, data: res.status === 204 ? null : await res.json() };
}

try {
  assert.equal((await call('POST', '/tournaments', { name: 'Test' })).status, 201);
  for (const name of ['Simone', 'Luca', 'Marco']) {
    assert.equal((await call('POST', '/players', { name, team: 'Ferrari' })).status, 201);
  }
  assert.equal((await call('POST', '/players', { name: 'simone' })).status, 409, 'nome duplicato');

  await call('POST', '/tournaments/1/races', { track: 'Monza', type: 'race' });
  await call('POST', '/tournaments/1/races', { track: 'Imola', type: 'sprint' });

  assert.equal((await call('PUT', '/races/1/results', { results: [
    { player_id: 1, position: 2 }, { player_id: 2, position: 1 }, { player_id: 3, dnf: true },
  ] })).status, 204);
  assert.equal((await call('PUT', '/races/2/results', { results: [
    { player_id: 1, position: 1 }, { player_id: 2, position: 3 }, { player_id: 3, position: 2 },
  ] })).status, 204);
  assert.equal((await call('PUT', '/races/2/results', { results: [
    { player_id: 1, position: 1 }, { player_id: 2, position: 1 },
  ] })).status, 400, 'posizione duplicata');
  assert.equal((await call('DELETE', '/players/1')).status, 409, 'pilota con risultati');

  const { data } = await call('GET', '/tournaments/1/standings');
  const summary = data.standings.map((s) => `${s.rank}. ${s.name} ${s.points}pt`);
  // Simone: 18 + 8 = 26, Luca: 25 + 6 = 31, Marco: 0 + 7 = 7
  assert.deepEqual(summary, ['1. Luca 31pt', '2. Simone 26pt', '3. Marco 7pt']);

  console.log('OK', summary.join(' | '));
} finally {
  server.kill();
  await new Promise((resolve) => server.once('exit', resolve));
  rmSync(dir, { recursive: true, force: true });
}
