import sys,json,uuid,threading; sys.path.insert(0,'/tmp/apitest')
from lib import *
import subprocess
def reset_login_limits():
    subprocess.run("docker exec goscaffold-redis redis-cli --scan --pattern 'rl:login*' | xargs -r docker exec -i goscaffold-redis redis-cli del",
                   shell=True,capture_output=True)

A=json.load(open('tok.json'))['admin']['access_token']

print("--- one refresh token used by 8 threads at once ---")
for trial in range(3):
    reset_login_limits(); pair=login('member@example.com','another correct horse')
    rt=pair['refresh_token']; out=[]
    def go():
        s,b,_=call("POST","/api/v1/auth/refresh",{"refresh_token":rt})
        out.append((s,b))
    ts=[threading.Thread(target=go) for _ in range(8)]
    for t in ts: t.start()
    for t in ts: t.join()
    ok=[x for x in out if x[0]==200]
    check(f"trial {trial}: at most one concurrent use succeeds (got {len(ok)} of 8)", len(ok)<=1,
          [x[0] for x in out])
    # every token minted in this race must be dead, since a reuse revokes the family
    live=0
    for s,b in ok:
        s2,_,_=call("POST","/api/v1/auth/refresh",{"refresh_token":json.loads(b)['refresh_token']})
        if s2==200: live+=1
    check(f"trial {trial}: the family is revoked after the race", live==0, f"{live} tokens still live")

print("\n--- the same email created by 6 threads at once ---")
for trial in range(2):
    email=f"race-{uuid.uuid4().hex[:10]}@example.com"; out=[]
    def mk():
        s,b,_=call("POST","/api/v1/users",{"email":email,"name":"Race","password":"a long enough password"},token=A)
        out.append(s)
    ts=[threading.Thread(target=mk) for _ in range(6)]
    for t in ts: t.start()
    for t in ts: t.join()
    check(f"trial {trial}: exactly one create wins (201s={out.count(201)})", out.count(201)==1, out)
    check(f"trial {trial}: the losers get 409, never 500", all(s in (201,409) for s in out), out)

print("\n--- concurrent logout and refresh ---")
reset_login_limits()
pair=login('member@example.com','another correct horse')
rt=pair['refresh_token']; res={}
def lo(): res['logout']=call("POST","/api/v1/auth/logout",{"refresh_token":rt})[0]
def rf(): res['refresh']=call("POST","/api/v1/auth/refresh",{"refresh_token":rt})[0]
t1,t2=threading.Thread(target=lo),threading.Thread(target=rf)
t1.start();t2.start();t1.join();t2.join()
check(f"no 500 from a logout/refresh race ({res})", all(v!=500 for v in res.values()), res)
s,_,_=call("POST","/api/v1/auth/refresh",{"refresh_token":rt})
check("the token is dead after logout", s==401, s)
report()
