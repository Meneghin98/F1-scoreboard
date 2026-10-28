-- Programma dei prossimi Gran Premi (vedi rotation.Plan): piste suggerite, da
-- giocare nell'ordine, come "11,13,5". Vuoto se non è mai stato generato.
ALTER TABLE tournaments ADD COLUMN track_plan TEXT NOT NULL DEFAULT '';
-- Gran Premi già corsi quando il programma è stato generato: la prima pista
-- vale per il Gran Premio successivo, e poi si avanza di una pista per ognuno.
ALTER TABLE tournaments ADD COLUMN track_plan_from INTEGER NOT NULL DEFAULT 0;

-- Piste da non suggerire (noiose o troppo difficili), scelte nella pagina
-- Piste, come "31,42": valgono per tutti i tornei.
INSERT INTO settings (key, value) VALUES ('excluded_tracks', '');
