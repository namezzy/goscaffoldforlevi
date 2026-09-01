import os,sys,json; sys.path.insert(0,os.path.dirname(os.path.abspath(__file__)))
from lib import *
T=load_tokens()
A=T['admin']['access_token']; M=T['member']['access_token']
import base64
def sub(t):
    p=t.split('.')[1]; p+='='*(-len(p)%4)
    return json.loads(base64.urlsafe_b64decode(p))['sub']
aid,mid=sub(A),sub(M)

# a second member, as the IDOR victim. The address is unique per run: reusing a
# fixed one meant falling back to a lookup that assumed the whole user table fit
# in one page, and it stopped fitting.
import uuid
s,b,_=call("POST","/api/v1/users",{"email":f"victim-{uuid.uuid4().hex[:8]}@example.com","name":"Victim","password":"yet another long password"},token=A)
if s!=201:
    raise SystemExit(f"could not create the victim user: {s} {b}")
vid=json.loads(b)['id']
print(f"admin={aid}\nmember={mid}\nvictim={vid}\n")

print("--- horizontal: member acting on another user ---")
s,b,_=call("GET",f"/api/v1/users/{vid}",token=M);            check("member cannot read another user", s==403, f"{s} {b}")
s,b,_=call("PATCH",f"/api/v1/users/{vid}",{"name":"pwned"},token=M); check("member cannot rename another user", s==403, f"{s} {b}")
s,b,_=call("PUT",f"/api/v1/users/{vid}/password",{"password":"attacker chosen pw 123"},token=M); check("member cannot change another user's password", s==403, f"{s} {b}")
s,b,_=call("DELETE",f"/api/v1/users/{vid}",token=M);         check("member cannot delete another user", s==403, f"{s} {b}")
s,b,_=call("PATCH",f"/api/v1/users/{vid}",{"status":"disabled"},token=M); check("member cannot disable another user", s==403, f"{s} {b}")

print("\n--- vertical: member acting on the admin ---")
s,b,_=call("PUT",f"/api/v1/users/{aid}/password",{"password":"attacker chosen pw 123"},token=M); check("member cannot take over the admin account", s==403, f"{s} {b}")
s,b,_=call("PATCH",f"/api/v1/users/{aid}",{"status":"disabled"},token=M); check("member cannot disable the admin", s==403, f"{s} {b}")
s,b,_=call("GET","/api/v1/users",token=M);                   check("member cannot list users", s==403, f"{s} {b}")
s,b,_=call("POST","/api/v1/users",{"email":"x@example.com","name":"X","password":"a long enough password"},token=M); check("member cannot create users", s==403, f"{s} {b}")

print("\n--- self-service boundaries ---")
s,b,_=call("GET",f"/api/v1/users/{mid}",token=M);            check("member can read itself", s==200, f"{s} {b}")
s,b,_=call("PATCH",f"/api/v1/users/{mid}",{"name":"Renamed"},token=M); check("member can rename itself", s==200, f"{s} {b}")
s,b,_=call("PATCH",f"/api/v1/users/{mid}",{"role":"admin"},token=M);   check("member cannot promote itself", s==403, f"{s} {b}")
s,b,_=call("PATCH",f"/api/v1/users/{mid}",{"name":"Ok","role":"admin"},token=M); check("a mixed patch with role is refused whole", s==403, f"{s} {b}")
s,b,_=call("PATCH",f"/api/v1/users/{mid}",{"status":"disabled"},token=M); check("member changing its own status", s in (200,403), f"{s} {b}")
print(f"     (self status change -> {s})")
s,b,_=call("GET",f"/api/v1/users/{mid}",token=M)
if s==200: print("     status now:",json.loads(b)['status'])

print("\n--- unauthenticated ---")
for m,p in [("GET","/api/v1/users"),("GET",f"/api/v1/users/{mid}"),("POST","/api/v1/users"),("PATCH",f"/api/v1/users/{mid}"),("DELETE",f"/api/v1/users/{mid}"),("PUT",f"/api/v1/users/{mid}/password")]:
    s,b,_=call(m,p,{"name":"x"} if m in("PATCH","POST","PUT") else None)
    check(f"{m} {p.replace(mid,'{id}')} without a token -> 401", s==401, f"{s} {b}")
report()
