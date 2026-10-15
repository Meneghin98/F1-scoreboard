import express from 'express';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { db, transaction } from './src/db.js';
import { POINTS, pointsFor, compareStandings } from './src/scoring.js';

const app = express();
const PORT = Number(process.env.PORT) || 3000;

app.use(express.json());
app.use(express.static(join(dirname(fileURLToPath(import.meta.url)), 'public')));

class HttpError extends Error {
  constructor(status, message) {
    super(message);
    this.status = status;
  }
}

function requireText(value, field) {
  const text = typeof value === 'string' ? value.trim() : '';
  if (!text) throw new HttpError(400, `Il campo "${field}" è obbligatorio`);
  return text;
}

function findOr404(sql, id, what) {
  const row = db.prepare(sql).get(Number(id));
  if (!row) throw new HttpError(404, `${what} non trovato`);
  return row;
}

// --- Configurazione -------------------------------------------------------

app.get('/api/points', (req, res) => res.json(POINTS));

// --- Tornei ---------------------------------------------------------------

app.get('/api/tournaments', (req, res) => {
  res.json(db.prepare('SELECT * FROM tournaments ORDER BY id DESC').all());
});

app.post('/api/tournaments', (req, res) => {
  const name = requireText(req.body.name, 'nome');
  const { lastInsertRowid } = db.prepare('INSERT INTO tournaments (name) VALUES (?)').run(name);
  res.status(201).json(db.prepare('SELECT * FROM tournaments WHERE id = ?').get(lastInsertRowid));
});

app.delete('/api/tournaments/:id', (req, res) => {
  findOr404('SELECT id FROM tournaments WHERE id = ?', req.params.id, 'Torneo');
  db.prepare('DELETE FROM tournaments WHERE id = ?').run(Number(req.params.id));
  res.status(204).end();
});

// --- Piloti ---------------------------------------------------------------

app.get('/api/players', (req, res) => {
  res.json(db.prepare('SELECT * FROM players ORDER BY name').all());
});

app.post('/api/players', (req, res) => {
  const name = requireText(req.body.name, 'nome');
  const team = typeof req.body.team === 'string' ? req.body.team.trim() || null : null;
  if (db.prepare('SELECT 1 FROM players WHERE name = ?').get(name)) {
    throw new HttpError(409, `Esiste già un pilota chiamato "${name}"`);
  }
  const { lastInsertRowid } = db.prepare('INSERT INTO players (name, team) VALUES (?, ?)').run(name, team);
  res.status(201).json(db.prepare('SELECT * FROM players WHERE id = ?').get(lastInsertRowid));
});

app.delete('/api/players/:id', (req, res) => {
  const id = Number(req.params.id);
  findOr404('SELECT id FROM players WHERE id = ?', id, 'Pilota');
  if (db.prepare('SELECT 1 FROM results WHERE player_id = ?').get(id)) {
    throw new HttpError(409, 'Il pilota ha già dei risultati registrati e non può essere eliminato');
  }
  db.prepare('DELETE FROM players WHERE id = ?').run(id);
  res.status(204).end();
});

// --- Gare -----------------------------------------------------------------

app.get('/api/tournaments/:id/races', (req, res) => {
  const tournament = findOr404('SELECT id FROM tournaments WHERE id = ?', req.params.id, 'Torneo');
  const races = db.prepare('SELECT * FROM races WHERE tournament_id = ? ORDER BY id').all(tournament.id);
  const results = db.prepare(`
    SELECT r.* FROM results r JOIN races ra ON ra.id = r.race_id
    WHERE ra.tournament_id = ?
  `).all(tournament.id);

  res.json(races.map((race) => ({
    ...race,
    results: results
      .filter((r) => r.race_id === race.id)
      .map((r) => ({
        player_id: r.player_id,
        position: r.position,
        dnf: Boolean(r.dnf),
        points: pointsFor(race.type, r.position, r.dnf),
      }))
      .sort((a, b) => (a.position ?? Infinity) - (b.position ?? Infinity)),
  })));
});

app.post('/api/tournaments/:id/races', (req, res) => {
  const tournament = findOr404('SELECT id FROM tournaments WHERE id = ?', req.params.id, 'Torneo');
  const track = requireText(req.body.track, 'circuito');
  const type = req.body.type === 'sprint' ? 'sprint' : 'race';
  const date = typeof req.body.date === 'string' && req.body.date ? req.body.date : null;
  const { lastInsertRowid } = db
    .prepare('INSERT INTO races (tournament_id, track, type, date) VALUES (?, ?, ?, ?)')
    .run(tournament.id, track, type, date);
  res.status(201).json(db.prepare('SELECT * FROM races WHERE id = ?').get(lastInsertRowid));
});

app.delete('/api/races/:id', (req, res) => {
  findOr404('SELECT id FROM races WHERE id = ?', req.params.id, 'Gara');
  db.prepare('DELETE FROM races WHERE id = ?').run(Number(req.params.id));
  res.status(204).end();
});

// Sostituisce in blocco i risultati di una gara.
// Body: { results: [{ player_id, position, dnf }] }
app.put('/api/races/:id/results', (req, res) => {
  const race = findOr404('SELECT * FROM races WHERE id = ?', req.params.id, 'Gara');
  const input = Array.isArray(req.body.results) ? req.body.results : null;
  if (!input) throw new HttpError(400, 'Formato risultati non valido');

  const seenPlayers = new Set();
  const seenPositions = new Set();
  const rows = input.map((r) => {
    const playerId = Number(r.player_id);
    const dnf = Boolean(r.dnf);
    const position = dnf ? null : Number(r.position);
    if (!db.prepare('SELECT 1 FROM players WHERE id = ?').get(playerId)) {
      throw new HttpError(400, `Pilota ${r.player_id} inesistente`);
    }
    if (seenPlayers.has(playerId)) throw new HttpError(400, 'Pilota ripetuto nei risultati');
    seenPlayers.add(playerId);
    if (!dnf) {
      if (!Number.isInteger(position) || position < 1) {
        throw new HttpError(400, 'Le posizioni devono essere numeri interi maggiori di zero');
      }
      if (seenPositions.has(position)) throw new HttpError(400, `Posizione ${position} assegnata più volte`);
      seenPositions.add(position);
    }
    return { playerId, position, dnf };
  });

  transaction(() => {
    db.prepare('DELETE FROM results WHERE race_id = ?').run(race.id);
    const insert = db.prepare('INSERT INTO results (race_id, player_id, position, dnf) VALUES (?, ?, ?, ?)');
    for (const r of rows) insert.run(race.id, r.playerId, r.position, r.dnf ? 1 : 0);
  });
  res.status(204).end();
});

// --- Classifica -----------------------------------------------------------

app.get('/api/tournaments/:id/standings', (req, res) => {
  const tournament = findOr404('SELECT * FROM tournaments WHERE id = ?', req.params.id, 'Torneo');
  const races = db.prepare('SELECT id, track, type, date FROM races WHERE tournament_id = ? ORDER BY id').all(tournament.id);
  const rows = db.prepare(`
    SELECT r.race_id, r.player_id, r.position, r.dnf, ra.type, p.name, p.team
    FROM results r
    JOIN races ra ON ra.id = r.race_id
    JOIN players p ON p.id = r.player_id
    WHERE ra.tournament_id = ?
  `).all(tournament.id);

  const byPlayer = new Map();
  for (const row of rows) {
    let s = byPlayer.get(row.player_id);
    if (!s) {
      s = { player_id: row.player_id, name: row.name, team: row.team,
            points: 0, wins: 0, podiums: 0, races: 0, dnf: 0, positions: [], perRace: {} };
      byPlayer.set(row.player_id, s);
    }
    const points = pointsFor(row.type, row.position, row.dnf);
    s.points += points;
    s.perRace[row.race_id] = { points, position: row.position, dnf: Boolean(row.dnf) };
    // Vittorie, podi e countback considerano solo i Gran Premi, non le sprint.
    if (row.type === 'race') {
      s.races++;
      if (row.dnf) {
        s.dnf++;
      } else {
        s.positions.push(row.position);
        if (row.position === 1) s.wins++;
        if (row.position <= 3) s.podiums++;
      }
    }
  }

  const standings = [...byPlayer.values()].sort(compareStandings).map(({ positions, ...s }, i) => ({ rank: i + 1, ...s }));
  res.json({ tournament, races, standings });
});

// --- Errori ---------------------------------------------------------------

app.use('/api', (req, res) => res.status(404).json({ error: 'Endpoint non trovato' }));

app.use((err, req, res, next) => {
  const status = err.status || (err.type === 'entity.parse.failed' ? 400 : 500);
  if (status === 500) console.error(err);
  res.status(status).json({ error: status === 500 ? 'Errore interno del server' : err.message });
});

app.listen(PORT, () => {
  console.log(`F1 Scoreboard in ascolto su http://localhost:${PORT}`);
});
