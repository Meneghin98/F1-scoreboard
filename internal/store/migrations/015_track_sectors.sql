-- Dove iniziano il settore 2 e il 3, in metri dal traguardo: il gioco non lo
-- dice e il server lo ricava dalla telemetria. 0 finché non è noto.
ALTER TABLE track_maps ADD COLUMN sector2_start REAL NOT NULL DEFAULT 0;
ALTER TABLE track_maps ADD COLUMN sector3_start REAL NOT NULL DEFAULT 0;
