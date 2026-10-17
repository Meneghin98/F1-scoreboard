-- Collega le registrazioni ai tornei e ai piloti.
--
-- Una registrazione può appartenere a una gara (e quindi a un torneo). Se è una
-- gara con la classifica finale, i risultati della gara vengono ricavati dalla
-- registrazione: per ogni giocatore umano, il pilota associato prende la sua
-- posizione d'arrivo.

ALTER TABLE telemetry_sessions ADD COLUMN race_id INTEGER REFERENCES races(id) ON DELETE SET NULL;
ALTER TABLE telemetry_sessions ADD COLUMN cars INTEGER;                      -- auto in pista
ALTER TABLE telemetry_sessions ADD COLUMN analyzed INTEGER NOT NULL DEFAULT 0; -- giocatori già estratti

CREATE INDEX telemetry_sessions_race ON telemetry_sessions (race_id);

-- Giocatori umani di una sessione, con il pilota del segnapunti che li guidava.
CREATE TABLE telemetry_players (
  session_id    INTEGER NOT NULL REFERENCES telemetry_sessions(id) ON DELETE CASCADE,
  car_index     INTEGER NOT NULL,
  name          TEXT NOT NULL,     -- nome mostrato dal gioco
  team          TEXT NOT NULL,
  race_number   INTEGER NOT NULL,
  position      INTEGER,           -- dalla classifica finale, NULL se non arrivata
  result_status INTEGER,
  player_id     INTEGER REFERENCES players(id) ON DELETE SET NULL,
  PRIMARY KEY (session_id, car_index)
);

CREATE INDEX telemetry_players_player ON telemetry_players (player_id);
