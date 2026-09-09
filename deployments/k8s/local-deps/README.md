# 本机集群的依赖

**这不是部署制品。** 这里的 PostgreSQL、Redis 与 NATS 只为在 kind / k3d /
minikube 上把 `overlays/local` 真正跑起来而存在:没有资源限制、没有持久卷、
没有密码轮换,数据存在 `emptyDir` 里,Pod 一重启就没了。

真实环境里这三样是托管服务或由平台团队维护的 StatefulSet,**不应该由应用的
manifest 创建** —— 这也是 `base/` 里没有它们的原因。

用法:

```sh
kubectl apply -k deployments/k8s/local-deps
kubectl -n goscaffold-local rollout status deploy/postgres deploy/redis deploy/nats
kubectl apply -k deployments/k8s/overlays/local
```

`overlays/local` 假设这三个 Service 的名字分别是 `postgres`、`redis`、`nats`,
与它 `secretGenerator` 里的连接串一致。改名字要两边一起改。
