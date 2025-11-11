Add "loki" plugin to docker:

```
docker plugin install grafana/loki-docker-driver:latest --alias loki --grant-all-permissions
```

Check installed plugins if needed:

```
docker plugin ls
```

Start applications stack:

```
docker compose up -d
```

Stop applications stack:

```
docker compose down
```

To test the web application visit coffee shop UI:

```
http://localhost:8888
```

