-- Dati della classifica finale per le statistiche: miglior giro, griglia, soste e penalità.

-- Giocatori di una sessione, dalla classifica finale (NULL se non arrivata).
ALTER TABLE telemetry_players ADD COLUMN best_lap_ms INTEGER;   -- miglior giro, NULL se nessun giro valido
ALTER TABLE telemetry_players ADD COLUMN grid_position INTEGER; -- posizione in griglia
ALTER TABLE telemetry_players ADD COLUMN laps INTEGER;          -- giri completati
ALTER TABLE telemetry_players ADD COLUMN pit_stops INTEGER;
ALTER TABLE telemetry_players ADD COLUMN penalties_s INTEGER;   -- secondi di penalità
ALTER TABLE telemetry_players ADD COLUMN race_time_ms INTEGER;  -- tempo totale, senza penalità

-- Miglior giro della sessione tra tutte le auto, IA comprese: dice chi ha fatto il giro più veloce.
ALTER TABLE telemetry_sessions ADD COLUMN best_lap_ms INTEGER;
-- 1 quando i dati qui sopra sono stati letti dalla classifica finale. Le sessioni
-- registrate prima vengono rilette all'avvio (vedi live.Service.backfillStats).
ALTER TABLE telemetry_sessions ADD COLUMN stats_extracted INTEGER NOT NULL DEFAULT 0;

-- Risultati delle gare: miglior giro (dalle registrazioni) e giro più veloce
-- della gara (dalle registrazioni o segnato a mano).
ALTER TABLE results ADD COLUMN best_lap_ms INTEGER;
ALTER TABLE results ADD COLUMN fastest_lap INTEGER NOT NULL DEFAULT 0;
