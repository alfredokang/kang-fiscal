#!/usr/bin/env python3
"""Gera pkg/sefaz/endpoints.json e pkg/sefaz/cstat.json a partir do sped-nfe (MIT).

    python3 -I scripts/sefaz-tables.py <sped-nfe/storage> pkg/sefaz

Os endereços mudam algumas vezes por ano e variam por UF, modelo e ambiente. Copiá-los à
mão é o caminho para o erro que só aparece como "falha de conexão" numa UF que ninguém
testou; gerar da fonte mais usada do país, e versionar o resultado, é o que dá para revisar.
"""
import json, sys, xml.etree.ElementTree as ET, datetime, os

src, out = sys.argv[1], sys.argv[2]

SERVICES = {
    "NfeStatusServico": "status", "NfeAutorizacao": "authorization",
    "NfeRetAutorizacao": "authorization_result", "NfeConsultaProtocolo": "protocol_query",
    "NfeInutilizacao": "void_range", "RecepcaoEvento": "event", "NfeConsultaCadastro": "registry_query",
    "NfeDistribuicaoDFe": "dfe_distribution", "RecepcaoEPEC": "epec",
}
ENVS = {"homologacao": "homologation", "producao": "production"}

def services(model):
    root = ET.parse(os.path.join(src, f"wsnfe_4.00_mod{model}.xml")).getroot()
    result = {}
    for uf in root.findall("UF"):
        sigla = uf.find("sigla").text.strip()
        for env_pt, env in ENVS.items():
            node = uf.find(env_pt)
            if node is None:
                continue
            for child in node:
                name = SERVICES.get(child.tag)
                url = (child.text or "").strip()
                if not name or not url:
                    continue
                result.setdefault(sigla, {}).setdefault(env, {})[name] = {
                    "url": url, "operation": child.get("operation"),
                    "method": child.get("method"), "version": child.get("version"),
                }
    return result

auth = json.load(open(os.path.join(src, "autorizadores.json")))
tables = {
    "source": "nfephp-org/sped-nfe storage/ (MIT), gerado em " + datetime.date.today().isoformat(),
    "authorizers": {"55": auth["55"], "65": auth["65"]},
    "services": {"55": services(55), "65": services(65)},
}
json.dump(tables, open(os.path.join(out, "endpoints.json"), "w"), ensure_ascii=False, indent=1, sort_keys=True)

cstat = json.load(open(os.path.join(src, "cstat.json")))
codes = {c["cod"]: {"message": c["msg"].strip(), "hint": (c.get("correct") or "").strip()} for c in cstat if c.get("cod")}
json.dump({"source": tables["source"], "codes": codes}, open(os.path.join(out, "cstat.json"), "w"),
          ensure_ascii=False, indent=1, sort_keys=True)
print(len(codes), "códigos;", sum(len(v) for m in tables["services"].values() for v in m.values()), "conjuntos de endereços")
