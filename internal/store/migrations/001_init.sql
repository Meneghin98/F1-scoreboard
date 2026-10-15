-- Schema iniziale, identico a quello della versione Node.js.
-- IF NOT EXISTS permette di adottare senza problemi i database già esistenti.

CREATE TABLE IF NOT EXISTS tournaments (
  id         INTEGER PRIMARY KEY,
  name       TEXT NOT NULL,
  created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS players (
  id   INTEGER PRIMARY KEY,
  name TEXT NOT NULL UNIQUE COLLATE NOCASE,
  team TEXT
);

CREATE TABLE IF NOT EXISTS races (
  id            INTEGER PRIMARY KEY,
  tournament_id INTEGER NOT NULL REFERENCES tournaments(id) ON DELETE CASCADE,
  track         TEXT NOT NULL,
  type          TEXT NOT NULL CHECK (type IN ('race', 'sprint')),
  date          TEXT
);

CREATE TABLE IF NOT EXISTS results (
  race_id   INTEGER NOT NULL REFERENCES races(id) ON DELETE CASCADE,
  player_id INTEGER NOT NULL REFERENCES players(id) ON DELETE RESTRICT,
  position  INTEGER,
  dnf       INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (race_id, player_id)
);
