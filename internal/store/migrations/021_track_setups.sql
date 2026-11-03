-- Assetto consigliato di ogni circuito, mostrato nella pagina Piste. Uno per
-- circuito (track_id è l'id del gioco, come in live.Tracks: le varianti
-- inverse hanno un id loro). Non c'è un'API da cui prenderli: si aggiungono
-- a mano con nuove migrazioni, per esempio
--
--   INSERT INTO track_setups (track_id, front_wing, rear_wing, ...) VALUES (11, 5, 8, ...);
--
-- I nomi seguono le sezioni dell'assetto di F1 25 e i valori come li mostra
-- il gioco.
CREATE TABLE track_setups (
    track_id INTEGER PRIMARY KEY,
    -- Aerodinamica
    front_wing INTEGER NOT NULL,
    rear_wing INTEGER NOT NULL,
    -- Trasmissione: differenziale in accelerazione e in rilascio, freno motore (%);
    -- il freno motore manca in alcuni assetti, e allora resta NULL
    on_throttle INTEGER NOT NULL,
    off_throttle INTEGER NOT NULL,
    engine_braking INTEGER,
    -- Geometria sospensioni: campanatura e convergenza (gradi)
    front_camber REAL NOT NULL,
    rear_camber REAL NOT NULL,
    front_toe REAL NOT NULL,
    rear_toe REAL NOT NULL,
    -- Sospensioni: rigidità, barre antirollio, altezza da terra
    front_suspension INTEGER NOT NULL,
    rear_suspension INTEGER NOT NULL,
    front_anti_roll_bar INTEGER NOT NULL,
    rear_anti_roll_bar INTEGER NOT NULL,
    front_height INTEGER NOT NULL,
    rear_height INTEGER NOT NULL,
    -- Freni: pressione e ripartizione verso l'anteriore (%)
    brake_pressure INTEGER NOT NULL,
    brake_bias INTEGER NOT NULL,
    -- Pneumatici: pressioni (PSI)
    front_left_pressure REAL NOT NULL,
    front_right_pressure REAL NOT NULL,
    rear_left_pressure REAL NOT NULL,
    rear_right_pressure REAL NOT NULL,
    -- Da dove viene l'assetto, consigli di guida: facoltativo
    notes TEXT NOT NULL DEFAULT ''
);
