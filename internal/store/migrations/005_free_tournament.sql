-- Torneo libero, torneo in corso e controller dei giocatori.
--
-- Le gare finite vengono salvate in automatico nel torneo in corso oppure, se
-- non ce n'è uno, nel torneo libero. Un torneo è in corso tra started_at ed
-- ended_at: ce n'è al massimo uno alla volta.

ALTER TABLE tournaments ADD COLUMN free INTEGER NOT NULL DEFAULT 0; -- 1 solo per il torneo libero
ALTER TABLE tournaments ADD COLUMN started_at TEXT;                 -- UTC, NULL se mai iniziato
ALTER TABLE tournaments ADD COLUMN ended_at TEXT;                   -- UTC, NULL se non terminato

CREATE UNIQUE INDEX tournaments_one_free ON tournaments (free) WHERE free = 1;
CREATE UNIQUE INDEX tournaments_one_active ON tournaments ((started_at IS NOT NULL AND ended_at IS NULL))
  WHERE started_at IS NOT NULL AND ended_at IS NULL;

INSERT INTO tournaments (name, free) VALUES ('Torneo libero', 1);

-- Controller che guidava l'auto in split-screen (1 o 2), NULL per gli altri
-- giocatori: serve a proporre lo stesso pilota alla gara successiva.
ALTER TABLE telemetry_players ADD COLUMN controller INTEGER;
