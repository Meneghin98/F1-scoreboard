-- Punti per posizione scelti dal torneo, usati solo in modalità 'players' (vedi
-- scoring.Rules): "10,6,3" dà 10 punti al primo, 6 al secondo e 3 al terzo.
-- Vuoto vale la tabella F1 2025, cioè il comportamento di prima.
ALTER TABLE tournaments ADD COLUMN race_points TEXT NOT NULL DEFAULT '';
ALTER TABLE tournaments ADD COLUMN sprint_points TEXT NOT NULL DEFAULT '';
