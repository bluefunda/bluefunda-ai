# Docker Deployment Example

This directory contains example files for deploying `abaper` using Docker.

## Quick Start

1. **Start the service:**
   ```bash
   docker-compose up -d
   ```

2. **Test the service:**
   ```bash
   curl http://localhost:8013/health
   ```

## SAP credentials

`abaper serve` has no SAP login of its own. Every request carries the caller's own SAP credentials as
headers, and a request without them is rejected with `401`:

```bash
curl -X POST http://localhost:8013/api/v1/objects/get \
  -H "Content-Type: application/json" \
  -H "X-SAP-Host: https://your-sap-system.company.com:8000" \
  -H "X-SAP-Client: 100" \
  -H "X-SAP-User: $SAP_USER" \
  -H "X-SAP-Password: $SAP_PASSWORD" \
  -d '{"object_type":"CLAS","object_name":"ZCL_EXAMPLE"}'
```

Do not configure `SAP_HOST`/`SAP_USERNAME`/`SAP_PASSWORD` for a shared server: they are ignored. A static
SAP identity (`abaper serve --allow-static-sap`) is for local, single-user use on your own machine only and
listens on `127.0.0.1`.

## Configuration

The example uses the published Docker image `ghcr.io/bluefunda/abaper:latest`.

| Variable | Description | Default |
|----------|-------------|---------|
| `LOG_LEVEL` | Logging level | `info` |

## Production Considerations

- Use Docker secrets or a proper secret management system for passwords
- Set up proper logging and monitoring
- Configure resource limits based on your needs
- Consider using a reverse proxy (nginx, traefik) for HTTPS termination

## Stopping

```bash
docker-compose down
```

To remove all data:
```bash
docker-compose down -v
```
