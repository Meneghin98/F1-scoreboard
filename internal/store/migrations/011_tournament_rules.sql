-- Regole di punteggio di ogni torneo (vedi scoring.Rules). I valori di default
-- sono il regolamento F1 2025, cioè il comportamento di prima.

-- 'absolute': i punti vanno in base alla posizione d'arrivo, IA compresa;
-- 'players': in ogni gara le posizioni si ricalcolano tra i soli piloti del segnapunti.
ALTER TABLE tournaments ADD COLUMN points_mode TEXT NOT NULL DEFAULT 'absolute' CHECK (points_mode IN ('absolute', 'players'));
-- Punti bonus al primo della qualifica e al giro più veloce dei Gran Premi (0 = nessun bonus).
ALTER TABLE tournaments ADD COLUMN pole_points INTEGER NOT NULL DEFAULT 0;
ALTER TABLE tournaments ADD COLUMN fastest_lap_points INTEGER NOT NULL DEFAULT 0;
