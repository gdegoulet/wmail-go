#!/bin/sh
# Vérifie les scripts Sieve produits par wmail-go avec le vrai interpréteur Pigeonhole (image Dovecot) :
# chaque script est compilé (sievec) puis exécuté sur des messages d'essai (sieve-test), et la décision est comparée à
# l'attendu. Aucun serveur ni boîte réelle n'est touché. Usage : tools/sievecheck/run.sh (depuis la racine du projet).
set -eu
cd "$(dirname "$0")/../.."
OUT="scratch/sievecheck"
IMG="${DOVECOT_IMAGE:-docker.io/dovecot/dovecot:latest}"
rm -rf "$OUT"; mkdir -p "$OUT"
docker compose --profile dev run --rm dev go run ./tools/sievecheck "/src/$OUT" 2>&1 | grep -v "Emulate\|compose provider" || true
fail=0; total=0
for s in "$OUT"/*.sieve; do
  base=$(basename "$s" .sieve)
  if ! docker run --rm --user 0 --entrypoint /dovecot/bin/sievec -v "$PWD/$OUT:/w:Z" "$IMG" -d "/w/$base.sieve" >/dev/null 2>"$OUT/$base.err"; then
    echo "ÉCHEC compilation : $base"; sed 's/^/    /' "$OUT/$base.err"; fail=$((fail+1)); continue
  fi
  python3 -I - "$OUT" "$base" <<'PY' > "$OUT/$base.msgs"
import json, sys
out, base = sys.argv[1:3]
for m in json.load(open(f"{out}/{base}.json")):
    print(m["name"].replace(" ", "_"))
PY
  while read -r name; do
    total=$((total+1))
    res=$(docker run --rm --user 0 --entrypoint /dovecot/bin/sieve-test -v "$PWD/$OUT:/w:Z" "$IMG" -t - "/w/$base.sieve" "/w/$base.$name.eml" 2>&1 || true)
    printf '%s' "$res" > "$OUT/$base.$name.out"
    if ! python3 -I - "$OUT" "$base" "$name" <<'PY'
import json, sys
out, base, name = sys.argv[1:4]
res = open(f"{out}/{base}.{name}.out", encoding="utf-8", errors="replace").read()
msgs = {m["name"].replace(" ", "_"): m for m in json.load(open(f"{out}/{base}.json"))}
m = msgs[name]
bad = []
if "final result: success" not in res:
    bad.append("résultat final non réussi")
for p in m.get("present") or []:
    if p not in res: bad.append(f"manque « {p} »")
for a in m.get("absent") or []:
    if a in res: bad.append(f"ne devrait pas contenir « {a} »")
if bad:
    print(f"ÉCHEC {base} / {name} : " + " ; ".join(bad)); print(res); sys.exit(1)
PY
    then fail=$((fail+1)); fi
  done < "$OUT/$base.msgs"
done
echo "$total essais, $fail échec(s)"
[ "$fail" -eq 0 ]
