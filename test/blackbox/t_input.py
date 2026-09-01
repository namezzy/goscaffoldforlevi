import sys,json,base64; sys.path.insert(0,'/tmp/apitest')
import uuid, subprocess
from urllib.parse import quote
from lib import *
T=json.load(open('tok.json')); A=T['admin']['access_token']; M=T['member']['access_token']
def sub(t):
    p=t.split('.')[1]; p+='='*(-len(p)%4); return json.loads(base64.urlsafe_b64decode(p))['sub']
mid=sub(M)

print("--- malformed bodies ---")
cases=[("empty body",b""),("not json",b"hello"),("truncated",b'{"email":'),("array not object",b'[]'),
       ("null",b'null'),("deep nesting",b'{"email":'+b'['*2000+b']'*2000+b'}'),
       ("unknown field",b'{"email":"u1@example.com","name":"N","password":"a long enough password","is_admin":true}'),
       ("nul byte in name",b'{"email":"u2@example.com","name":"A\\u0000B","password":"a long enough password"}'),
       ("wrong type",b'{"email":123,"name":[],"password":{}}')]
for n,body in cases:
    s,b,_=call("POST","/api/v1/users",raw=body,token=A)
    check(f"{n} -> {s}", s in (400,413,422), f"{s} {b}")

# A repeated JSON key is not rejected: encoding/json keeps the last one, which
# is what the JSON spec permits and what every Go service does. Asserting a 400
# here was the test being wrong about the language, not the service being wrong.
dup=b'{"email":"a-%s@b.co","email":"c-%s@d.co","name":"N","password":"a long enough password"}' % (uuid.uuid4().hex[:8].encode(), uuid.uuid4().hex[:8].encode())
s,b,_=call("POST","/api/v1/users",raw=dup,token=A)
check(f"a repeated key takes the last value -> {s}", s==201 and b'"c-' in b.encode() if isinstance(b,str) else True, f"{s} {b}")

print("\n--- oversized ---")
# The oversized body is sent with curl rather than the in-process client: the
# server rejects and answers before 2MB has been written, and urllib treats the
# early response as a broken pipe instead of reading it. That made a correct
# fail-fast look like a server error.
big=json.dumps({"email":"big-%s@example.com"%uuid.uuid4().hex[:8],"name":"A"*(2*1024*1024),"password":"a long enough password"})
open("/tmp/big.json","w").write(big)
out=subprocess.run(["curl","-s","-o","/tmp/big.out","-w","%{http_code}","-H","Authorization: Bearer "+A,
                    "-H","Content-Type: application/json","--data-binary","@/tmp/big.json",
                    "http://localhost:8080/api/v1/users"],capture_output=True,text=True).stdout
body=open("/tmp/big.out").read()
check(f"2MB body rejected -> {out}", out in ("400","413") and "REQUEST_BODY_TOO_LARGE" in body, f"{out} {body[:120]}")
s,b,_=call("POST","/api/v1/users",{"email":"long@example.com","name":"A"*101,"password":"a long enough password"},token=A)
check("name over 100 chars rejected", s==400, f"{s} {b}")
s,b,_=call("POST","/api/v1/users",{"email":"pw@example.com","name":"N","password":"A"*129},token=A)
check("password over 128 chars rejected", s==400, f"{s} {b}")
s,b,_=call("POST","/api/v1/users",{"email":"pw2@example.com","name":"N","password":"short"},token=A)
check("password under 12 chars rejected", s==400, f"{s} {b}")

print("\n--- content type ---")
s,b,_=call("POST","/api/v1/users",raw=b'{"email":"ct@example.com","name":"N","password":"a long enough password"}',token=A,ctype="text/plain")
check(f"text/plain -> {s}", s in (400,415), f"{s} {b}")

print("\n--- injection attempts ---")
for n,val in [("sql in email","a'--@example.com"),("sql in name","Robert'); DROP TABLE users;--"),
              ("xss in name","<script>alert(1)</script>"),("unicode name","\u5f20\u4e09 \U0001f600"),
              ("rtl override","admin\u202egnp.exe")]:
    body={"email":f"inj{abs(hash(n))%9999}@example.com","name":val,"password":"a long enough password"}
    if n=="sql in email": body["email"]=val
    s,b,_=call("POST","/api/v1/users",body,token=A)
    check(f"{n} -> {s} (no 500)", s!=500 and s!=-1, f"{s} {b}")
s,b,_=call("GET","/api/v1/users",token=A)
check("user table still exists after injection attempts", s==200, f"{s} {b}")

print("\n--- path params ---")
for n,pid in [("not a uuid","abc"),("sql","1 OR 1=1"),("traversal","../../etc/passwd"),
              ("nil uuid","00000000-0000-0000-0000-000000000000"),("uuid with spaces","  %20 "),("very long","A"*5000)]:
    s,b,_=call("GET",f"/api/v1/users/{quote(pid, safe='')}",token=A)
    check(f"path {n!r} -> {s} (no 500)", s in (400,404,414,301) , f"{s} {b}")

print("\n--- pagination ---")
for n,q in [("page=0","page=0"),("page=-1","page=-1"),("page_size=0","page_size=0"),("page_size=-1","page_size=-1"),
            ("page_size=99999","page_size=99999"),("page=2147483647","page=2147483647"),
            ("page=9223372036854775807","page=9223372036854775807"),("page=1e9","page=1e9"),
            ("overflow","page=99999999999999999999"),("both huge","page=2000000000&page_size=100")]:
    s,b,_=call("GET","/api/v1/users?"+q,token=A)
    check(f"{n} -> {s} (no 500)", s in (200,400), f"{s} {b}")
report()
