-- Assetti consigliati per asciutto, controller, controllo trazione spento, ABS
-- acceso, Gran Premio con 1 giro di qualifica e gara di 5 giri. Ricavati il
-- 04/11/2026 dagli assetti di F1 26 su F1Laps (https://www.f1laps.com/f1-26/setups/):
-- - dal Meta Setup del circuito, cioè dove concordano i 100 assetti più veloci:
--   campanatura, convergenza, altezze, sospensione anteriore, pressione dei
--   freni e le ali quando il Meta le segna come importanti;
-- - dalla mediana dei 5 assetti più veloci (diversi tra loro) con controller,
--   TC spento e ABS acceso, in qualifica, gara o time trial: differenziale,
--   barre antirollio, sospensione posteriore, ripartizione dei freni, pressioni
--   delle gomme e le altre ali. Sono le regolazioni che cambiano con gli aiuti.
-- Con 5 giri l'usura delle gomme non conta: la geometria resta la più spinta.
-- Mancano Spa, che ha il suo (022), e le piste inverse, che F1Laps non ha.
INSERT INTO track_setups (track_id, front_wing, rear_wing, on_throttle, off_throttle, front_camber, rear_camber, front_toe, rear_toe, front_suspension, rear_suspension, front_anti_roll_bar, rear_anti_roll_bar, front_height, rear_height, brake_pressure, brake_bias, front_left_pressure, front_right_pressure, rear_left_pressure, rear_right_pressure, notes) VALUES
    (0, 25, 25, 100, 35, -3.50, -2.00, 0.00, 0.10, 37, 19, 5, 11, 22, 48, 100, 56, 29.5, 29.5, 22.0, 22.0,
     'Da F1Laps (04/11/2026): geometria, altezze, sospensione anteriore e freni dal Meta Setup (ali comprese, 100 assetti più veloci); il resto dai 5 migliori assetti con controller, TC off e ABS on.'),
    (2, 46, 32, 80, 30, -3.50, -2.00, 0.00, 0.10, 40, 11, 3, 10, 22, 44, 100, 55, 27.5, 27.5, 22.5, 22.5,
     'Da F1Laps (04/11/2026): geometria, altezze, sospensione anteriore e freni dal Meta Setup (100 assetti più veloci); il resto dai 5 migliori assetti con controller, TC off e ABS on.'),
    (3, 39, 28, 75, 50, -3.50, -2.00, 0.00, 0.10, 38, 12, 8, 19, 20, 47, 100, 55, 26.5, 26.5, 20.5, 20.5,
     'Da F1Laps (04/11/2026): geometria, altezze, sospensione anteriore e freni dal Meta Setup (ali comprese, 100 assetti più veloci); il resto dai 5 migliori assetti con controller, TC off e ABS on. Differenziale in accelerazione al 75% invece del 90% del Meta: senza TC serve più trazione in uscita dalle curve lente.'),
    (4, 44, 36, 85, 40, -3.50, -2.00, 0.00, 0.10, 39, 15, 15, 12, 22, 46, 100, 54, 24.2, 24.2, 20.9, 20.9,
     'Da F1Laps (04/11/2026): geometria, altezze, sospensione anteriore e freni dal Meta Setup (100 assetti più veloci); il resto dai 5 migliori assetti con controller, TC off e ABS on.'),
    (5, 50, 50, 60, 20, -3.50, -2.00, 0.00, 0.10, 38, 4, 6, 3, 22, 51, 100, 56, 22.7, 22.7, 20.5, 20.5,
     'Da F1Laps (04/11/2026): geometria, altezze, sospensione anteriore e freni dal Meta Setup (ali comprese, 100 assetti più veloci); il resto dai 5 migliori assetti con controller, TC off e ABS on. Differenziale in accelerazione al 60% invece del 75% del Meta: senza TC serve più trazione in uscita dalle curve lente.'),
    (6, 45, 24, 65, 35, -3.50, -2.00, 0.00, 0.10, 38, 4, 8, 18, 21, 46, 100, 54, 26.8, 26.8, 20.5, 20.5,
     'Da F1Laps (04/11/2026): geometria, altezze, sospensione anteriore e freni dal Meta Setup (100 assetti più veloci); il resto dai 5 migliori assetti con controller, TC off e ABS on. Differenziale in accelerazione al 65% invece del 90% del Meta: senza TC serve più trazione in uscita dalle curve lente.'),
    (7, 21, 12, 80, 40, -3.50, -2.00, 0.00, 0.10, 41, 9, 9, 8, 21, 48, 100, 56, 25.4, 25.4, 23.0, 23.0,
     'Da F1Laps (04/11/2026): geometria, altezze, sospensione anteriore e freni dal Meta Setup (100 assetti più veloci); il resto dai 5 migliori assetti con controller, TC off e ABS on. Differenziale in accelerazione al 80% invece del 100% del Meta: senza TC serve più trazione in uscita dalle curve lente.'),
    (9, 50, 45, 65, 25, -3.50, -2.00, 0.00, 0.10, 41, 11, 10, 14, 22, 45, 100, 57, 28.0, 28.0, 23.7, 23.7,
     'Da F1Laps (04/11/2026): geometria, altezze, sospensione anteriore e freni dal Meta Setup (ali comprese, 100 assetti più veloci); il resto dai 5 migliori assetti con controller, TC off e ABS on. Differenziale in accelerazione al 65% invece del 100% del Meta: senza TC serve più trazione in uscita dalle curve lente.'),
    (11, 12, 0, 100, 30, -3.50, -2.00, 0.00, 0.10, 41, 2, 5, 18, 23, 48, 99, 56, 29.5, 29.5, 23.6, 23.6,
     'Da F1Laps (04/11/2026): geometria, altezze, sospensione anteriore e freni dal Meta Setup (ali comprese, 100 assetti più veloci); il resto dai 5 migliori assetti con controller, TC off e ABS on.'),
    (12, 50, 50, 85, 35, -3.50, -2.00, 0.00, 0.10, 41, 3, 6, 12, 19, 48, 100, 57, 23.3, 23.3, 20.9, 20.9,
     'Da F1Laps (04/11/2026): geometria, altezze, sospensione anteriore e freni dal Meta Setup (ali comprese, 100 assetti più veloci); il resto dai 5 migliori assetti con controller, TC off e ABS on. Differenziale in accelerazione al 85% invece del 100% del Meta: senza TC serve più trazione in uscita dalle curve lente.'),
    (13, 44, 22, 90, 40, -3.50, -2.00, 0.00, 0.10, 41, 10, 4, 10, 23, 46, 100, 55, 27.8, 27.8, 21.9, 21.9,
     'Da F1Laps (04/11/2026): geometria, altezze, sospensione anteriore e freni dal Meta Setup (100 assetti più veloci); il resto dai 5 migliori assetti con controller, TC off e ABS on.'),
    (14, 46, 23, 80, 30, -3.50, -2.00, 0.00, 0.10, 40, 16, 15, 11, 21, 46, 100, 54, 26.5, 26.5, 20.5, 20.5,
     'Da F1Laps (04/11/2026): geometria, altezze, sospensione anteriore e freni dal Meta Setup (ali comprese, 100 assetti più veloci); il resto dai 5 migliori assetti con controller, TC off e ABS on. Differenziale in accelerazione al 80% invece del 100% del Meta: senza TC serve più trazione in uscita dalle curve lente.'),
    (15, 45, 36, 90, 35, -3.50, -2.00, 0.00, 0.10, 38, 11, 12, 21, 20, 46, 100, 54, 28.0, 28.0, 20.5, 20.5,
     'Da F1Laps (04/11/2026): geometria, altezze, sospensione anteriore e freni dal Meta Setup (ali comprese, 100 assetti più veloci); il resto dai 5 migliori assetti con controller, TC off e ABS on.'),
    (16, 40, 25, 70, 35, -3.50, -2.00, 0.00, 0.10, 41, 16, 7, 11, 22, 46, 100, 55, 29.5, 29.5, 23.0, 23.0,
     'Da F1Laps (04/11/2026): geometria, altezze, sospensione anteriore e freni dal Meta Setup (ali comprese, 100 assetti più veloci); il resto dai 5 migliori assetti con controller, TC off e ABS on. Differenziale in accelerazione al 70% invece del 92.5% del Meta: senza TC serve più trazione in uscita dalle curve lente.'),
    (17, 36, 18, 80, 35, -3.50, -2.00, 0.00, 0.10, 41, 8, 8, 15, 22, 47, 100, 57, 25.5, 25.5, 21.5, 21.5,
     'Da F1Laps (04/11/2026): geometria, altezze, sospensione anteriore e freni dal Meta Setup (ali comprese, 100 assetti più veloci); il resto dai 5 migliori assetti con controller, TC off e ABS on.'),
    (19, 50, 34, 75, 30, -3.50, -2.00, 0.00, 0.10, 37, 16, 13, 13, 23, 44, 100, 55, 24.2, 24.2, 21.9, 21.9,
     'Da F1Laps (04/11/2026): il Meta Setup del circuito non è leggibile, quindi tutto viene dai 5 migliori assetti con controller, TC off e ABS on.'),
    (20, 30, 12, 70, 30, -3.50, -2.00, 0.00, 0.10, 38, 16, 5, 8, 22, 46, 100, 57, 24.3, 24.3, 21.5, 21.5,
     'Da F1Laps (04/11/2026): geometria, altezze, sospensione anteriore e freni dal Meta Setup (100 assetti più veloci); il resto dai 5 migliori assetti con controller, TC off e ABS on.'),
    (26, 50, 38, 90, 30, -3.50, -2.00, 0.00, 0.10, 41, 11, 11, 16, 21, 49, 100, 55, 24.2, 24.2, 20.9, 20.9,
     'Da F1Laps (04/11/2026): geometria, altezze, sospensione anteriore e freni dal Meta Setup (ali comprese, 100 assetti più veloci); il resto dai 5 migliori assetti con controller, TC off e ABS on.'),
    (27, 50, 31, 80, 50, -3.50, -2.00, 0.00, 0.10, 41, 10, 4, 6, 22, 48, 100, 55, 28.0, 28.0, 23.5, 23.5,
     'Da F1Laps (04/11/2026): geometria, altezze, sospensione anteriore e freni dal Meta Setup (ali comprese, 100 assetti più veloci); il resto dai 4 migliori assetti con controller, TC off e ABS on. Differenziale in accelerazione al 80% invece del 100% del Meta: senza TC serve più trazione in uscita dalle curve lente.'),
    (29, 23, 10, 85, 45, -3.50, -2.00, 0.00, 0.10, 41, 10, 3, 11, 22, 45, 100, 56, 26.5, 26.5, 21.6, 21.6,
     'Da F1Laps (04/11/2026): geometria, altezze, sospensione anteriore e freni dal Meta Setup (ali comprese, 100 assetti più veloci); il resto dai 5 migliori assetti con controller, TC off e ABS on. Differenziale in accelerazione al 85% invece del 100% del Meta: senza TC serve più trazione in uscita dalle curve lente.'),
    (30, 36, 22, 70, 50, -3.50, -2.00, 0.00, 0.10, 41, 4, 3, 12, 23, 48, 100, 57, 26.0, 26.0, 21.5, 21.5,
     'Da F1Laps (04/11/2026): geometria, altezze, sospensione anteriore e freni dal Meta Setup (100 assetti più veloci); il resto dai 5 migliori assetti con controller, TC off e ABS on. Differenziale in accelerazione al 70% invece del 100% del Meta: senza TC serve più trazione in uscita dalle curve lente.'),
    (31, 12, 3, 80, 40, -3.50, -2.00, 0.00, 0.10, 39, 10, 5, 7, 23, 46, 100, 56, 25.5, 25.5, 21.5, 21.5,
     'Da F1Laps (04/11/2026): geometria, altezze, sospensione anteriore e freni dal Meta Setup (100 assetti più veloci); il resto dai 5 migliori assetti con controller, TC off e ABS on. Differenziale in accelerazione al 80% invece del 95% del Meta: senza TC serve più trazione in uscita dalle curve lente.'),
    (32, 50, 25, 100, 40, -3.50, -2.00, 0.00, 0.10, 39, 15, 8, 10, 22, 47, 100, 54, 29.2, 29.2, 20.5, 20.5,
     'Da F1Laps (04/11/2026): geometria, altezze, sospensione anteriore e freni dal Meta Setup (100 assetti più veloci); il resto dai 5 migliori assetti con controller, TC off e ABS on.'),
    (42, 50, 35, 75, 35, -3.50, -2.00, 0.00, 0.10, 41, 2, 7, 11, 22, 51, 100, 57, 24.9, 24.9, 21.5, 21.5,
     'Da F1Laps (04/11/2026): geometria, altezze, sospensione anteriore e freni dal Meta Setup (ali comprese, 100 assetti più veloci); il resto dai 5 migliori assetti con controller, TC off e ABS on. Differenziale in accelerazione al 75% invece del 100% del Meta: senza TC serve più trazione in uscita dalle curve lente.');
