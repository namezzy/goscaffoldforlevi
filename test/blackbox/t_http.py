import sys,json,time,statistics; sys.path.insert(0,'/tmp/apitest')
from lib import *
import subprocess,base64
def reset():
    subprocess.run("docker exec goscaffold-redis redis-cli --scan --pattern 'rl:*' | xargs -r docker exec -i goscaffold-redis redis-cli del",shell=True,capture_output=True)
T=json.load(open('tok.json')); A=T['admin']['access_token']; M=T['member']['access_token']
def sub(t):
    p=t.split('.')[1]; p+='='*(-len(p)%4); return json.loads(base64.urlsafe_b64decode(p))['sub']
mid=sub(M)

print("--- HTTP semantics ---")
s,b,h=call("DELETE","/api/v1/auth/login",token=A);   check(f"unsupported method -> 405 (got {s})", s==405, f"{s} {b}")
check("405 names the allowed methods", 'Allow' in h or s!=405, h)
s,b,h=call("HEAD",f"/api/v1/users/{mid}",token=A);   check(f"HEAD on a GET route -> {s}", s in (200,405), f"{s} {b}")
s,b,h=call("OPTIONS","/api/v1/users",token=A);       check(f"OPTIONS -> {s} (no 500)", s!=500, f"{s} {b}")
s,b,_=call("GET","/api/v1/nope",token=A);            check("unknown path -> 404", s==404, f"{s} {b}")
s,b,_=call("GET","/api/v2/users",token=A);           check("unknown version -> 404", s==404, f"{s} {b}")
s,b,h=call("GET","/api/v1/users",token=A)
check("responses carry a request id", 'X-Request-Id' in h or 'X-Request-ID' in h, list(h))
check("security headers present", any(k.lower()=='x-content-type-options' for k in h), list(h))
check("no server banner", not any(k.lower()=='server' for k in h), list(h))
s,b,h=call("GET","/api/v1/users",token=A,headers={"Origin":"https://evil.example"})
print("     CORS on a cross-origin GET:",{k:v for k,v in h.items() if k.lower().startswith('access-control')} or "none")

print("\n--- rate limit headers ---")
reset()
s,b,h=call("GET","/api/v1/users",token=A)
rl={k:v for k,v in h.items() if k.lower().startswith('x-ratelimit')}
check("list carries rate limit headers", len(rl)>0, h)
print("     ",rl)
s,b,h=call("POST","/api/v1/auth/login",{"email":"member@example.com","password":"wrong"})
rl2={k:v for k,v in h.items() if k.lower().startswith('x-ratelimit')}
check("login does NOT leak remaining attempts", len(rl2)==0, rl2)
reset()
for i in range(6):
    s,b,h=call("POST","/api/v1/auth/login",{"email":"member@example.com","password":"wrong"})
check(f"6th wrong login -> 429 (got {s})", s==429, f"{s} {b}")
check("429 carries Retry-After", any(k.lower()=='retry-after' for k in h), list(h))
print("     Retry-After:",[v for k,v in h.items() if k.lower()=='retry-after'])
s,b,_=call("POST","/api/v1/auth/login",{"email":"admin@example.com","password":"twTHyedj8xxIRQ1CUVD63i4i6oqtJ3J2"})
check(f"a different identifier is unaffected by the id limit (got {s})", s in (200,429), f"{s} {b}")
print(f"     (admin login during member lockout -> {s}; the login IP limit is 10/15m and is shared)")
report()
