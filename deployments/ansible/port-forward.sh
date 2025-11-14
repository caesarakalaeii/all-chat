#!/bin/bash
echo "Starting port forwards..."
kubectl port-forward -n allchat svc/api-gateway 8080:8080 &
kubectl port-forward -n allchat svc/postgres 5432:5432 &
kubectl port-forward -n allchat svc/redis 6379:6379 &
echo "Port forwards active. Press Ctrl+C to stop."
wait
