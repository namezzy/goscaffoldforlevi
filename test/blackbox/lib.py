import json,urllib.request,urllib.error,time,base64,hmac,hashlib,os,sys

# Everything the suite needs to reach a running service. Hardcoding a
# password meant the suite broke the moment the account was rotated, and it
# broke with a 401 that looked like an authorization bug rather than a
# stale fixture.
BASE=os.environ.get("BLACKBOX_BASE_URL","http://localhost:8080")
ADMIN_EMAIL=os.environ.get("BLACKBOX_ADMIN_EMAIL","admin@example.com")
ADMIN_PW=os.environ.get("BLACKBOX_ADMIN_PASSWORD","")
TOKENS=os.environ.get("BLACKBOX_TOKENS","tok.json")
def call(method,path,body=None,token=None,headers=None,raw=None,ctype="application/json"):
    data=raw if raw is not None else (json.dumps(body).encode() if body is not None else None)
    if isinstance(data,str): data=data.encode()
    req=urllib.request.Request(BASE+path,data=data,method=method)
    if data is not None: req.add_header("Content-Type",ctype)
    if token: req.add_header("Authorization","Bearer "+token)
    for k,v in (headers or {}).items(): req.add_header(k,v)
    try:
        with urllib.request.urlopen(req,timeout=15) as r:
            return r.status, r.read().decode(), dict(r.headers)
    except urllib.error.HTTPError as e:
        return e.code, e.read().decode(), dict(e.headers)
    except Exception as e:
        return -1, repr(e), {}
def login(email,pw):
    s,b,_=call("POST","/api/v1/auth/login",{"email":email,"password":pw})
    if s!=200: raise SystemExit(f"login {email} failed: {s} {b}")
    return json.loads(b)
def b64u(b): return base64.urlsafe_b64encode(b).rstrip(b"=").decode()
def jwt(payload,secret,alg="HS256",header=None):
    h=header or {"alg":alg,"typ":"JWT"}
    hp=b64u(json.dumps(h).encode())+"."+b64u(json.dumps(payload).encode())
    if alg=="none": return hp+"."
    sig=hmac.new(secret.encode(),hp.encode(),hashlib.sha256).digest()
    return hp+"."+b64u(sig)
PASS=[];FAIL=[]
def check(name,cond,detail=""):
    (PASS if cond else FAIL).append(name)
    print(("  ok   " if cond else "  FAIL ")+name+((" :: "+str(detail)[:220]) if (detail and not cond) else ""))
def report():
    print(f"\n=== {len(PASS)} passed, {len(FAIL)} failed ===")
    for f in FAIL: print("  FAILED:",f)


def load_tokens():
    """Read the tokens written by the bootstrap step."""
    try:
        return json.load(open(TOKENS))
    except FileNotFoundError:
        raise SystemExit(
            f"{TOKENS} not found. Run `python3 lib.py` first, with "
            "BLACKBOX_ADMIN_PASSWORD set to the administrator's password.")


def bootstrap():
    """Log in as the administrator, create a fresh member, and save both tokens.

    The member is created per run with a unique address. Reusing a fixed one
    made the suite depend on state left by the previous run, so a failure
    halfway through poisoned every run after it.
    """
    if not ADMIN_PW:
        raise SystemExit(
            "set BLACKBOX_ADMIN_PASSWORD to the administrator's password")

    admin = login(ADMIN_EMAIL, ADMIN_PW)
    member_email = "member-%d@example.com" % int(time.time() * 1000)
    member_pw = "correct horse battery staple"

    status, body, _ = call("POST", "/api/v1/users", {
        "email": member_email,
        "name": "Blackbox Member",
        "password": member_pw,
        "role": "user",
    }, token=admin["access_token"])
    if status != 201:
        raise SystemExit(f"creating the member account failed: {status} {body}")

    member = login(member_email, member_pw)
    with open(TOKENS, "w") as fh:
        json.dump({"admin": admin, "member": member}, fh)
    print(f"wrote {TOKENS} (admin={ADMIN_EMAIL}, member={member_email})")


if __name__ == "__main__":
    bootstrap()
