# Black-box suite

Adversarial tests that run against a **running** service over HTTP. They exist
because the Go tests share the service's own assumptions: a fake repository that
silently accepts a negative offset cannot reveal that the offset overflowed, and
a handler test that never sets Content-Type cannot reveal that it was never
checked. Everything here was found by treating the API as a stranger would.

## Running

```sh
make run                       # or: go run ./cmd/api
python3 lib.py                 # writes tok.json (admin + member tokens)
python3 t_jwt.py "$AUTH_JWT_SECRET"   # or export it; argv wins
python3 t_authz.py
python3 t_input.py
python3 t_race.py
python3 t_http.py
python3 t_fixed.py
```

Login is rate limited at 5 attempts per identifier per 15 minutes, so clear the
counters between runs:

```sh
docker exec goscaffold-redis redis-cli --scan --pattern 'rl:*' \
  | xargs -r docker exec -i goscaffold-redis redis-cli del
```

| Script | Covers |
|---|---|
| `t_jwt.py` | alg=none, tampered payload, wrong key, exp/nbf/iss/aud |
| `t_authz.py` | IDOR, vertical escalation, self-promotion, anonymous access |
| `t_input.py` | malformed and oversized bodies, injection, numeric overflow |
| `t_race.py` | refresh double-spend, duplicate signup, logout/refresh races |
| `t_http.py` | method and content negotiation, security and rate-limit headers |
| `t_fixed.py` | regression cover for every defect this suite has found |

## Two rules learned the hard way

**Prove the baseline is accepted before claiming a rejection means anything.**
The first JWT run passed all 18 assertions while signing with the wrong
variable name — `JWT_SECRET` instead of `AUTH_JWT_SECRET`. Every 401 was just a
bad signature, so not one assertion tested the claim it named.

**A request the client fails to send is not a rejection by the server.** An
unencoded space in a path and a 2MB body that the server answers mid-upload
both surfaced as client errors, and both looked like server defects until the
same request was replayed with curl.
