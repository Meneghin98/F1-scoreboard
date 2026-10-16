-- Piante dei circuiti ricostruite dalla telemetria: la prima volta che si corre
-- su un circuito il server la ricava dalle posizioni delle auto, le volte dopo
-- la mappa è disponibile da subito.

CREATE TABLE track_maps (
  track_id     INTEGER PRIMARY KEY, -- i layout invertiti hanno un id diverso
  track_length INTEGER NOT NULL,    -- metri, come nel pacchetto Session
  step         INTEGER NOT NULL,    -- metri tra un punto e il successivo
  points       TEXT NOT NULL,       -- JSON: un {x, z} per tratto, null se mai visto
  updated_at   TEXT NOT NULL        -- UTC, come datetime('now')
);
