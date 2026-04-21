up:
	docker-compose up -d --build

logs: down up
	docker compose logs -f

down:
	docker-compose down

test:
	@curl -s http://localhost:8080/ \
  -H "RPC-Service: root" \
  -H "RPC-Procedure: call" \
  -H "RPC-Caller: curl" \
  -H "RPC-Encoding: raw" \
  -H "Context-TTL-MS: 2000" \
  -d "test body" -D - -o /dev/null | grep "X-Error-Tree" | grep -o '{.*}' | jq .

test-success:
	@curl -v http://localhost:8080/ \
  -H "RPC-Service: root" \
  -H "RPC-Procedure: call" \
  -H "RPC-Caller: curl" \
  -H "RPC-Encoding: raw" \
  -H "Context-TTL-MS: 2000" \
  -H "Rpc-Header-x-error-bypass: true" \
  -d "test body"

test-fail-open:
	@curl -s http://localhost:8080/ \
  -H "RPC-Service: root" \
  -H "RPC-Procedure: call" \
  -H "RPC-Caller: curl" \
  -H "RPC-Encoding: raw" \
  -H "Context-TTL-MS: 2000" \
  -H "Rpc-Header-x-force-pass-root: true" \
  -d "test body" -D - -o /dev/null | grep "X-Error-Tree" | grep -o '{.*}' | jq .
