FROM postgres:16-alpine

COPY migrations/*.sql /migrations/
COPY scripts/run-migrations.sh /run-migrations.sh
RUN chmod +x /run-migrations.sh

ENTRYPOINT ["/run-migrations.sh"]
