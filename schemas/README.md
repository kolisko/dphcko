# Oficiální EPO schémata

Repozitář obsahuje neměnné, gzipované a Base64 zakódované kopie oficiálních XSD:

- `DPHDP3` 03.01.03 ze dne 9. 3. 2026
- `DPHKH1` 03.01.14 ze dne 9. 3. 2026

Zdrojové adresy jsou `https://adisspr.mfcr.cz/adis/jepo/schema/dphdp3_epo2.xsd`
a `https://adisspr.mfcr.cz/adis/jepo/schema/dphkh1_epo2.xsd`. Skript
`scripts/validate-xsd.sh` před validací ověří SHA-256 obou rozbalených schémat.
Zakódování drží velké strojově generované XML mimo běžný diff, vlastní obsah XSD
se tím nemění.
