import sys,json,time,base64; sys.path.insert(0,'/tmp/apitest')
from lib import *
SECRET=sys.argv[1]
T=json.load(open('tok.json'))
admin_at=T['admin']['access_token']; member_at=T['member']['access_token']
member_rt=T['member']['refresh_token']

def payload_of(tok):
    p=tok.split('.')[1]; p+='='*(-len(p)%4)
    return json.loads(base64.urlsafe_b64decode(p))

print("--- JWT forgery ---")
mp=payload_of(member_at)
print("  member claims:",{k:v for k,v in mp.items() if k not in('jti',)})

# 1 alg=none
s,b,_=call("GET","/api/v1/users",token=jwt({**mp,"role":"admin"},SECRET,alg="none"))
check("alg=none is rejected", s==401, f"{s} {b}")

# 2 signature stripped
s,b,_=call("GET","/api/v1/users",token=".".join(member_at.split('.')[:2])+".")
check("empty signature is rejected", s==401, f"{s} {b}")

# 3 signed with the wrong key
s,b,_=call("GET","/api/v1/users",token=jwt({**mp,"role":"admin"},"an-attacker-chosen-secret-value-x"))
check("wrong signing key is rejected", s==401, f"{s} {b}")

# 4 payload tampered, original signature kept
h,p,sig=member_at.split('.')
tampered=h+"."+b64u(json.dumps({**mp,"role":"admin"}).encode())+"."+sig
s,b,_=call("GET","/api/v1/users",token=tampered)
check("tampered payload with the old signature is rejected", s==401, f"{s} {b}")

# 5 correctly signed but role escalated  -> the real test of whether role is trusted from the token
esc=jwt({**mp,"role":"admin"},SECRET)
s,b,_=call("GET","/api/v1/users",token=esc)
check("a validly signed token IS trusted for role (documented design)", s==200, f"{s} {b}")

# 6 expired
s,b,_=call("GET","/api/v1/users",token=jwt({**mp,"exp":int(time.time())-60},SECRET))
check("expired token is rejected", s==401, f"{s} {b}")

# 7 not yet valid
s,b,_=call("GET","/api/v1/users",token=jwt({**mp,"nbf":int(time.time())+3600,"iat":int(time.time())+3600},SECRET))
check("future nbf is rejected", s==401, f"{s} {b}")

# 8 wrong issuer
s,b,_=call("GET","/api/v1/users",token=jwt({**mp,"iss":"evil"},SECRET))
check("wrong issuer is rejected", s==401, f"{s} {b}")

# 9 wrong audience
s,b,_=call("GET","/api/v1/users",token=jwt({**mp,"aud":"some-other-api"},SECRET))
check("wrong audience is rejected", s==401, f"{s} {b}")

# 10 no exp at all
noexp={k:v for k,v in mp.items() if k!='exp'}
s,b,_=call("GET","/api/v1/users",token=jwt(noexp,SECRET))
check("token without exp is rejected", s==401, f"{s} {b}")

# 11 refresh token used as a bearer
s,b,_=call("GET","/api/v1/users",token=member_rt)
check("refresh token is not usable as an access token", s==401, f"{s} {b}")

# 12 access token used as a refresh token
s,b,_=call("POST","/api/v1/auth/refresh",{"refresh_token":member_at})
check("access token is not usable as a refresh token", s==401, f"{s} {b}")

# 13 header smuggling
for h in ["Basic abc","bearer "+member_at,"Bearer  "+member_at,"Bearer "+member_at+" x",member_at]:
    s,b,_=call("GET","/api/v1/users/"+mp['sub'],headers={"Authorization":h})
    check(f"auth header variant {h[:14]!r} -> {s}", s in (200,401), f"{s} {b}")

# 14 subject that does not exist, validly signed
s,b,_=call("GET","/api/v1/users",token=jwt({**mp,"sub":"01a05000-0000-7000-8000-000000000000","role":"admin"},SECRET))
check("validly signed token for a non-existent user", s in (200,401,403), f"{s} {b}")
print("     (note: %d — stateless access tokens do not hit the user table, per ADR-0007)" % s)
report()
