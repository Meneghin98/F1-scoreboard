-- La turnazione dei controller si può spegnere per un torneo (vedi il
-- pacchetto rotation): di default è attiva, come prima.
ALTER TABLE tournaments ADD COLUMN rotation INTEGER NOT NULL DEFAULT 1;
