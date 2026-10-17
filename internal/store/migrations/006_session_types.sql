-- Prove libere e qualifiche nei tornei, e "Nessun torneo" al posto del torneo libero.
--
-- Ogni sessione del weekend finita (con la classifica finale) appartiene a un
-- torneo: le prove libere e le qualifiche diventano gare di tipo 'practice' e
-- 'qualifying', che mostrano i piazzamenti ma non danno punti.

-- Per cambiare il CHECK sul tipo SQLite richiede di ricreare la tabella
-- (le migrazioni girano senza chiavi esterne, vedi store.migrate).
CREATE TABLE races_new (
  id            INTEGER PRIMARY KEY,
  tournament_id INTEGER NOT NULL REFERENCES tournaments(id) ON DELETE CASCADE,
  track         TEXT NOT NULL,
  type          TEXT NOT NULL CHECK (type IN ('race', 'sprint', 'qualifying', 'practice')),
  date          TEXT
);
INSERT INTO races_new (id, tournament_id, track, type, date) SELECT id, tournament_id, track, type, date FROM races;
DROP TABLE races;
ALTER TABLE races_new RENAME TO races;

-- Il torneo libero raccoglie le gare giocate fuori dai tornei: il nome lo dice in chiaro.
UPDATE tournaments SET name = 'Nessun torneo' WHERE free = 1 AND name = 'Torneo libero';

-- Sessioni finite registrate prima di questa versione e mai assegnate a un
-- torneo: all'avvio il server le mette in "Nessun torneo" (una volta sola,
-- così una gara eliminata a mano non ricompare).
ALTER TABLE telemetry_sessions ADD COLUMN needs_tournament INTEGER NOT NULL DEFAULT 0;
UPDATE telemetry_sessions SET needs_tournament = 1
WHERE race_id IS NULL AND has_classification = 1 AND ended_at IS NOT NULL AND session_type BETWEEN 1 AND 17;
