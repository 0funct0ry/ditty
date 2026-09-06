# TLS certificates for the `shell-tls` compose profile

This directory is empty on purpose. Before running
`docker compose --profile shell-tls up`, put a certificate and key here:

```bash
openssl req -x509 -newkey rsa:2048 -nodes -days 365 \
  -keyout examples/certs/tls.key -out examples/certs/tls.crt \
  -subj "/CN=localhost"
```

The `shell-tls` service expects `tls.crt` and `tls.key` in this directory.
