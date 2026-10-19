-- Sessioni del weekend con la classifica finale non ancora salvate in un torneo.
--
-- Il server segna la sessione appena arriva la classifica finale e toglie il
-- segno quando la salva nel torneo (o quando la si assegna a mano). Se si
-- spegne di colpo nel frattempo, al riavvio salva lui le sessioni rimaste segnate.
ALTER TABLE telemetry_sessions ADD COLUMN pending_save INTEGER NOT NULL DEFAULT 0;
