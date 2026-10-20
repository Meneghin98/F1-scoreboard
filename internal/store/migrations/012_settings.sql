-- Impostazioni del server modificabili dalla pagina Sistema, come coppie chiave → valore.
CREATE TABLE settings (
  key   TEXT PRIMARY KEY,
  value TEXT NOT NULL
);

-- Conservazione delle registrazioni (vedi live.Retention): di default i file
-- restano per sempre; se si sceglie un numero di giorni, quelli delle gare dei
-- tornei restano comunque.
INSERT INTO settings (key, value) VALUES
  ('recordings_retention_days', '0'),
  ('recordings_keep_tournaments', '1');
