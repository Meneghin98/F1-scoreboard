-- Colore di ogni pilota, lo stesso in tutti i grafici, nella classifica live e sulla mappa.
ALTER TABLE players ADD COLUMN color TEXT; -- "#rrggbb"

-- Ai piloti già inseriti si danno i colori della tavolozza (api.playerColors)
-- nell'ordine in cui sono stati creati.
UPDATE players SET color = CASE (SELECT COUNT(*) FROM players p WHERE p.id < players.id) % 10
  WHEN 0 THEN '#e8443a'
  WHEN 1 THEN '#3987e5'
  WHEN 2 THEN '#f5a524'
  WHEN 3 THEN '#22b573'
  WHEN 4 THEN '#a970ff'
  WHEN 5 THEN '#ff6fae'
  WHEN 6 THEN '#2fc6d6'
  WHEN 7 THEN '#9ccc3c'
  WHEN 8 THEN '#ff8a3d'
  WHEN 9 THEN '#7f8cff'
END;
