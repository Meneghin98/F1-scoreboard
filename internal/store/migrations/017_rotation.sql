-- Turnazione dei controller (vedi il pacchetto rotation).

-- Ordine dei piloti sorteggiato all'avvio del torneo, come "4,1,3,2": decide
-- la prima coppia e le parità successive. Vuoto se non è mai stato sorteggiato.
ALTER TABLE tournaments ADD COLUMN rotation_order TEXT NOT NULL DEFAULT '';

-- Piloti presenti, scelti nella pagina Piloti, come "1,4,5".
INSERT INTO settings (key, value) VALUES ('present_players', '');
