-- Tabella dei punti per posizione scelta dal torneo in modalità 'players' (vedi
-- scoring.Presets): 'f1', 'rank_down', 'all_or_nothing', o 'custom' per i punti
-- di race_points e sprint_points. Senza vincolo CHECK, così aggiungerne una non
-- richiede una migrazione: i valori li controlla il server.
ALTER TABLE tournaments ADD COLUMN points_preset TEXT NOT NULL DEFAULT 'f1';
-- I tornei con i punti già scelti a mano restano così.
UPDATE tournaments SET points_preset = 'custom' WHERE race_points <> '' OR sprint_points <> '';
