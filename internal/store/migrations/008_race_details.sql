-- Circuito delle gare come identificativo del gioco, risultati modificati a mano e indici.

-- Circuito del gioco (telemetry.TrackID), NULL se il nome non corrisponde a un
-- circuito noto: serve a riconoscere le gare in calendario e per le statistiche.
ALTER TABLE races ADD COLUMN track_id INTEGER;

-- 1 se i risultati sono stati modificati a mano: la registrazione collegata
-- non li riscrive più finché non si ripristinano.
ALTER TABLE races ADD COLUMN manual_results INTEGER NOT NULL DEFAULT 0;

CREATE INDEX races_tournament ON races (tournament_id);
CREATE INDEX results_player ON results (player_id);

-- Le gare create dalle registrazioni prendono il circuito dalla sessione,
-- quelle inserite a mano dal nome (quelli proposti dal segnapunti finora).
UPDATE races SET track_id = (
  SELECT s.track_id FROM telemetry_sessions s WHERE s.race_id = races.id AND s.track_id IS NOT NULL
  ORDER BY s.id DESC LIMIT 1
) WHERE track_id IS NULL;

UPDATE races SET track_id = CASE track
  WHEN 'Australia – Melbourne' THEN 0
  WHEN 'Cina – Shanghai' THEN 2
  WHEN 'Bahrain – Sakhir' THEN 3
  WHEN 'Spagna – Barcellona' THEN 4
  WHEN 'Monaco' THEN 5
  WHEN 'Canada – Montréal' THEN 6
  WHEN 'Gran Bretagna – Silverstone' THEN 7
  WHEN 'Ungheria – Hungaroring' THEN 9
  WHEN 'Belgio – Spa-Francorchamps' THEN 10
  WHEN 'Italia – Monza' THEN 11
  WHEN 'Singapore – Marina Bay' THEN 12
  WHEN 'Giappone – Suzuka' THEN 13
  WHEN 'Abu Dhabi – Yas Marina' THEN 14
  WHEN 'USA – Austin' THEN 15
  WHEN 'Brasile – Interlagos' THEN 16
  WHEN 'Austria – Red Bull Ring' THEN 17
  WHEN 'Messico – Città del Messico' THEN 19
  WHEN 'Azerbaijan – Baku' THEN 20
  WHEN 'Olanda – Zandvoort' THEN 26
  WHEN 'Emilia-Romagna – Imola' THEN 27
  WHEN 'Arabia Saudita – Jeddah' THEN 29
  WHEN 'Miami' THEN 30
  WHEN 'Las Vegas' THEN 31
  WHEN 'Qatar – Lusail' THEN 32
END WHERE track_id IS NULL;
