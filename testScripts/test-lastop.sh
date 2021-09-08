curl -X 'GET' \
  "http://broker:broker@localhost:3000/v2/service_instances/41cab85-d688-4dc5-bc5e-5264262207ab/last_operation?service_id=50f64495-80f6-42a2-8be4-e4a597416a9e&plan_id=e26e8478-9f7c-4974-8e70-5453fc2a1dd6&operation=$1" \
  -H 'accept: application/json' \
  -H 'X-Broker-API-Version: 2.16'