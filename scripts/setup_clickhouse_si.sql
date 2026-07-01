-- ClickHouse Setup for Slovenian Addresses
-- This script creates the table structure for Slovenian address data

CREATE TABLE IF NOT EXISTS default.slovenian_addresses
(
    `feature_id` String,
    `eid_naslov` String,
    `obcina_sifra` String,
    `obcina_naziv` String,
    `obcina_naziv_dj` String,
    `naselje_sifra` String,
    `naselje_naziv` String,
    `naselje_naziv_dj` String,
    `ulica_sifra` String,
    `ulica_naziv` String,
    `ulica_naziv_dj` String,
    `postni_okolis_sifra` String,
    `postni_okolis_naziv` String,
    `postni_okolis_naziv_dj` String,
    `hs_stevilka` String,
    `hs_dodatek` String,
    `st_stanovanja` String,
    `e` String,
    `n` String,
    `eid_obcina` String,
    `eid_naselje` String,
    `eid_ulica` String,
    `eid_postni_okolis` String,
    `eid_hisna_stevilka` String,
    `eid_stanovanje` String,
    `eid_stavba` String,
    `eid_cetrtna_skupnost` String,
    `eid_dz_volisce` String,
    `eid_krajevna_skupnost` String,
    `eid_lokalno_volisce` String,
    `eid_lokalna_volilna_enota` String,
    `eid_solski_okolis` String,
    `eid_statisticna_regija` String,
    `eid_upravna_enota` String,
    `eid_vaska_skupnost` String,
    `eid_volilna_enota_dz` String,
    `eid_volilni_okraj` String,
    `eid_kohezijska_regija` String,
    `datum_sys` String
)
ENGINE = MergeTree()
ORDER BY (naselje_naziv, postni_okolis_sifra, ulica_naziv)
SETTINGS index_granularity = 8192;

-- Verify table was created
SELECT 'Table created successfully' AS status;

