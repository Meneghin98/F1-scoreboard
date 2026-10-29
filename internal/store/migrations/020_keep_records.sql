-- La conservazione delle registrazioni non tiene più quelle delle gare dei
-- tornei (sono quasi tutte), ma quelle che detengono un record di un circuito
-- (vedi live.Retention). Si parte tenendole, qualunque fosse la scelta di prima.
DELETE FROM settings WHERE key = 'recordings_keep_tournaments';
INSERT INTO settings (key, value) VALUES ('recordings_keep_records', '1');
