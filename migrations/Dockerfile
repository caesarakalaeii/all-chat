# run-migrations.sh only needs psql and pg_isready. The postgres server image
# shipped a gosu built with an EOL Go toolchain; the client package does not.
FROM alpine:3.24

RUN apk --no-cache add postgresql17-client && psql --version && pg_isready --version

COPY migrations/*.sql /migrations/
COPY scripts/run-migrations.sh /run-migrations.sh
RUN chmod +x /run-migrations.sh

ENTRYPOINT ["/run-migrations.sh"]
