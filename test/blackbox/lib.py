import json,urllib.request,urllib.error,time,base64,hmac,hashlib
BASE="http://localhost:8080"
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
