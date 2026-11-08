#!/usr/bin/env bash
# Regera pkg/ibge/municipios.csv a partir da API de localidades do IBGE.
set -euo pipefail
curl -sf 'https://servicodados.ibge.gov.br/api/v1/localidades/municipios?view=nivelado' | python3 -c '
import json, sys
rows = sorted({(str(m["municipio-id"]), m["municipio-nome"], m["UF-sigla"]) for m in json.load(sys.stdin)})
sys.stdout.write("".join(";".join(r) + "\n" for r in rows))
' > pkg/ibge/municipios.csv
wc -l pkg/ibge/municipios.csv
