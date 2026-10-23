-- Riavvii e flashback delle sessioni, mostrati nella pagina Gare.
-- NULL finché non sono contati: le sessioni registrate prima vengono rilette
-- all'avvio (vedi live.Service.backfillCounters).
ALTER TABLE telemetry_sessions ADD COLUMN restarts INTEGER;   -- volte che la sessione è stata ricominciata
ALTER TABLE telemetry_sessions ADD COLUMN flashbacks INTEGER; -- flashback usati
