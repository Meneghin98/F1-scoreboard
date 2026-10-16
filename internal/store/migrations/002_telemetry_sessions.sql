-- Sessioni di telemetria registrate automaticamente dal server.
-- Una riga per registrazione: se il server riparte a sessione in corso, la
-- stessa sessione del gioco (session_uid) può avere più righe.

CREATE TABLE telemetry_sessions (
  id                 INTEGER PRIMARY KEY,
  session_uid        TEXT NOT NULL,     -- identificativo del gioco, in esadecimale
  packet_format      INTEGER NOT NULL,  -- 2025 o 2026
  track_id           INTEGER,           -- noti dal primo pacchetto Session
  session_type       INTEGER,
  total_laps         INTEGER,
  started_at         TEXT NOT NULL,     -- UTC, come datetime('now')
  ended_at           TEXT,              -- NULL finché la registrazione è in corso
  file               TEXT NOT NULL,     -- relativo alla cartella delle registrazioni
  size_bytes         INTEGER NOT NULL DEFAULT 0,
  packets            INTEGER NOT NULL DEFAULT 0,
  has_classification INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX telemetry_sessions_uid ON telemetry_sessions (session_uid);
