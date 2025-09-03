**CloudWatch Logs Insights**    
region: us-west-2    
log-group-prefixes:     
log-class: STANDARD   
account-identifiers: All   
start-time: -300s    
end-time: 0s    
query-string:
  ```
  fields @timestamp, @message
| sort @timestamp desc
| limit 1000

  ```
---
| @timestamp | @message |
| --- | --- |
| 2025-09-03 06:04:35.446 | END RequestId: 616be141-3353-4bdd-99de-e5b120029b07 |
| 2025-09-03 06:04:35.446 | REPORT RequestId: 616be141-3353-4bdd-99de-e5b120029b07 Duration: 4.17 ms Billed Duration: 5 ms Memory Size: 128 MB Max Memory Used: 37 MB |
| 2025-09-03 06:04:35.445 | 2025/09/03 06:04:35 Successfully processed node , created 0 edges from 0 candidates |
| 2025-09-03 06:04:35.442 | START RequestId: 616be141-3353-4bdd-99de-e5b120029b07 Version: $LATEST |
| 2025-09-03 06:04:35.442 | 2025/09/03 06:04:35 Processing NodeCreated event for node  with 0 keywords |
| 2025-09-03 06:04:35.336 | END RequestId: b26cab26-7bab-4fa1-9bc2-4ba73fc180ad |
| 2025-09-03 06:04:35.336 | REPORT RequestId: b26cab26-7bab-4fa1-9bc2-4ba73fc180ad Duration: 21.55 ms Billed Duration: 22 ms Memory Size: 128 MB Max Memory Used: 49 MB |
| 2025-09-03 06:04:35.329 | END RequestId: 749bea50-6aa7-4f49-b1c6-8787040c607f |
| 2025-09-03 06:04:35.329 | REPORT RequestId: 749bea50-6aa7-4f49-b1c6-8787040c607f Duration: 5.30 ms Billed Duration: 6 ms Memory Size: 128 MB Max Memory Used: 50 MB |
| 2025-09-03 06:04:35.328 | 2025-09-03T06:04:35.327Z DEBUG zap/logger.go:31 Retrieved nodes for user {"user_id": "125deabf-b32e-4313-b893-4a3ddb416cc2", "count": 4, "total": 4} |
| 2025-09-03 06:04:35.328 | 2025-09-03T06:04:35.327Z DEBUG zap/logger.go:31 Counter incremented {"metric": "query.success.query_GetNodesByUser", "value": 3} |
| 2025-09-03 06:04:35.328 | 2025-09-03T06:04:35.327Z DEBUG zap/logger.go:31 Histogram value recorded {"metric": "query.duration.query_GetNodesByUser", "value": 2} |
| 2025-09-03 06:04:35.328 | 2025-09-03T06:04:35.327Z DEBUG zap/logger.go:31 Query completed {"query": "GetNodesByUser", "duration": "2.821345ms"} |
| 2025-09-03 06:04:35.328 | 2025-09-03T06:04:35.327Z DEBUG zap/logger.go:31 Counter incremented {"metric": "query.success.query_GetNodesByUser", "value": 4} |
| 2025-09-03 06:04:35.328 | 2025-09-03T06:04:35.327Z DEBUG zap/logger.go:31 Histogram value recorded {"metric": "query.duration.query_GetNodesByUser", "value": 2} |
| 2025-09-03 06:04:35.328 | 2025/09/03 06:04:35 [169.254.4.101/xD3s5fXJwl-000007] "GET http://f51uq07z1g.execute-api.us-west-2.amazonaws.com/api/v1/nodes?limit=50 HTTP/1.1" from 73.140.251.80 - 200 1003B in 2.93981ms |
| 2025-09-03 06:04:35.328 | 2025/09/03 06:04:35 Post-cold-start request completed in 3.050898ms: GET /api/v1/nodes -> 200 (cold start age: 14.606742461s) |
| 2025-09-03 06:04:35.324 | START RequestId: 749bea50-6aa7-4f49-b1c6-8787040c607f Version: $LATEST |
| 2025-09-03 06:04:35.324 | 2025/09/03 06:04:35 Processing POST-COLD-START request (14.606742461s after cold start): GET /api/v1/nodes |
| 2025-09-03 06:04:35.324 | 2025-09-03T06:04:35.324Z DEBUG zap/logger.go:31 Executing query {"query": "GetNodesByUser"} |
| 2025-09-03 06:04:35.323 | END RequestId: 8622642c-caea-47a8-805c-9602b2b93509 |
| 2025-09-03 06:04:35.323 | REPORT RequestId: 8622642c-caea-47a8-805c-9602b2b93509 Duration: 6.50 ms Billed Duration: 7 ms Memory Size: 128 MB Max Memory Used: 49 MB |
| 2025-09-03 06:04:35.322 | 2025/09/03 06:04:35 [169.254.6.221/80i4fWdDpR-000004] "GET http://f51uq07z1g.execute-api.us-west-2.amazonaws.com/api/v1/categories HTTP/1.1" from 73.140.251.80 - 200 18B in 5.268161ms |
| 2025-09-03 06:04:35.322 | 2025/09/03 06:04:35 Post-cold-start request completed in 5.364567ms: GET /api/v1/categories -> 200 (cold start age: 14.552511454s) |
| 2025-09-03 06:04:35.320 | 2025-09-03T06:04:35.320Z DEBUG zap/logger.go:31 Retrieved graph {"user_id": "125deabf-b32e-4313-b893-4a3ddb416cc2", "node_id": "", "nodes": 4, "edges": 0} |
| 2025-09-03 06:04:35.320 | 2025-09-03T06:04:35.320Z DEBUG zap/logger.go:31 Counter incremented {"metric": "query.success.query_GetGraph", "value": 1} |
| 2025-09-03 06:04:35.320 | 2025-09-03T06:04:35.320Z DEBUG zap/logger.go:31 Histogram value recorded {"metric": "query.duration.query_GetGraph", "value": 4} |
| 2025-09-03 06:04:35.320 | 2025-09-03T06:04:35.320Z DEBUG zap/logger.go:31 Query completed {"query": "GetGraph", "duration": "4.504025ms"} |
| 2025-09-03 06:04:35.320 | 2025-09-03T06:04:35.320Z DEBUG zap/logger.go:31 Counter incremented {"metric": "query.success.query_GetGraph", "value": 2} |
| 2025-09-03 06:04:35.320 | 2025-09-03T06:04:35.320Z DEBUG zap/logger.go:31 Histogram value recorded {"metric": "query.duration.query_GetGraph", "value": 4} |
| 2025-09-03 06:04:35.320 | 2025/09/03 06:04:35 DEBUG: GetGraphData succeeded, graph has 4 nodes and 0 edges |
| 2025-09-03 06:04:35.320 | 2025/09/03 06:04:35 DEBUG: GetGraphData completed successfully - returning 4 elements |
| 2025-09-03 06:04:35.320 | 2025/09/03 06:04:35 [169.254.30.153/cOHUgATgGc-000012] "GET http://f51uq07z1g.execute-api.us-west-2.amazonaws.com/api/v1/graph-data HTTP/1.1" from 73.140.251.80 - 200 425B in 4.812894ms |
| 2025-09-03 06:04:35.320 | 2025/09/03 06:04:35 Post-cold-start request completed in 4.899486ms: GET /api/v1/graph-data -> 200 (cold start age: 14.534759083s) |
| 2025-09-03 06:04:35.317 | START RequestId: 8622642c-caea-47a8-805c-9602b2b93509 Version: $LATEST |
| 2025-09-03 06:04:35.317 | 2025/09/03 06:04:35 Processing POST-COLD-START request (14.552511454s after cold start): GET /api/v1/categories |
| 2025-09-03 06:04:35.317 | 2025/09/03 06:04:35 DEBUG: ListCategories called for userID: 125deabf-b32e-4313-b893-4a3ddb416cc2 |
| 2025-09-03 06:04:35.315 | START RequestId: b26cab26-7bab-4fa1-9bc2-4ba73fc180ad Version: $LATEST |
| 2025-09-03 06:04:35.315 | 2025/09/03 06:04:35 Processing POST-COLD-START request (14.534759083s after cold start): GET /api/v1/graph-data |
| 2025-09-03 06:04:35.315 | 2025/09/03 06:04:35 DEBUG: GetGraphData called for userID: 125deabf-b32e-4313-b893-4a3ddb416cc2 |
| 2025-09-03 06:04:35.315 | 2025/09/03 06:04:35 DEBUG: Calling queryBus.Send with GetGraphDataQuery |
| 2025-09-03 06:04:35.315 | 2025-09-03T06:04:35.315Z DEBUG zap/logger.go:31 Executing query {"query": "GetGraph"} |
| 2025-09-03 06:04:35.301 | END RequestId: ca15b8ae-c97b-4eed-abdf-39981fc8fe3e |
| 2025-09-03 06:04:35.301 | REPORT RequestId: ca15b8ae-c97b-4eed-abdf-39981fc8fe3e Duration: 4.71 ms Billed Duration: 5 ms Memory Size: 128 MB Max Memory Used: 37 MB |
| 2025-09-03 06:04:35.299 | 2025/09/03 06:04:35 Successfully processed node , created 0 edges from 0 candidates |
| 2025-09-03 06:04:35.296 | START RequestId: ca15b8ae-c97b-4eed-abdf-39981fc8fe3e Version: $LATEST |
| 2025-09-03 06:04:35.296 | 2025/09/03 06:04:35 Processing NodeCreated event for node  with 0 keywords |
| 2025-09-03 06:04:35.277 | END RequestId: 9c1a99a2-3203-4f2a-8e2c-44b8bab05b61 |
| 2025-09-03 06:04:35.277 | REPORT RequestId: 9c1a99a2-3203-4f2a-8e2c-44b8bab05b61 Duration: 8.26 ms Billed Duration: 9 ms Memory Size: 128 MB Max Memory Used: 49 MB |
| 2025-09-03 06:04:35.276 | 2025/09/03 06:04:35 Processing POST-COLD-START request (14.495629601s after cold start): POST /api/v1/nodes/97c8425495446a0a/categories |
| 2025-09-03 06:04:35.276 | 2025/09/03 06:04:35 DEBUG: CategorizeNode called for userID: 125deabf-b32e-4313-b893-4a3ddb416cc2 |
| 2025-09-03 06:04:35.276 | 2025/09/03 06:04:35 [169.254.30.153/cOHUgATgGc-000011] "POST http://f51uq07z1g.execute-api.us-west-2.amazonaws.com/api/v1/nodes/97c8425495446a0a/categories HTTP/1.1" from 73.140.251.80 - 200 98B in 94.355µs |
| 2025-09-03 06:04:35.276 | 2025/09/03 06:04:35 Post-cold-start request completed in 435.51µs: POST /api/v1/nodes/97c8425495446a0a/categories -> 200 (cold start age: 14.495629601s) |
| 2025-09-03 06:04:35.269 | START RequestId: 9c1a99a2-3203-4f2a-8e2c-44b8bab05b61 Version: $LATEST |
| 2025-09-03 06:04:35.199 | END RequestId: 51bc105a-c4d5-43bc-8130-6ac18fa12f0f |
| 2025-09-03 06:04:35.199 | REPORT RequestId: 51bc105a-c4d5-43bc-8130-6ac18fa12f0f Duration: 83.92 ms Billed Duration: 84 ms Memory Size: 128 MB Max Memory Used: 49 MB |
| 2025-09-03 06:04:35.196 | 2025/09/03 06:04:35 [169.254.30.153/cOHUgATgGc-000010] "POST http://f51uq07z1g.execute-api.us-west-2.amazonaws.com/api/v1/nodes HTTP/1.1" from 73.140.251.80 - 201 125B in 80.37001ms |
| 2025-09-03 06:04:35.196 | 2025/09/03 06:04:35 Post-cold-start request completed in 80.580937ms: POST /api/v1/nodes -> 201 (cold start age: 14.335328886s) |
| 2025-09-03 06:04:35.176 | 2025-09-03T06:04:35.176Z DEBUG zap/logger.go:31 Counter incremented {"metric": "node.created.user_id_125deabf-b32e-4313-b893-4a3ddb416cc2.has_tags_false", "value": 3} |
| 2025-09-03 06:04:35.176 | 2025-09-03T06:04:35.176Z INFO zap/logger.go:36 Node created successfully {"node_id": "7e2ac84b-8ae5-43c6-baf1-d057f07279f4", "user_id": "125deabf-b32e-4313-b893-4a3ddb416cc2"} |
| 2025-09-03 06:04:35.176 | 2025-09-03T06:04:35.176Z DEBUG zap/logger.go:31 Counter incremented {"metric": "command.success.command_CreateNode", "value": 5} |
| 2025-09-03 06:04:35.176 | 2025-09-03T06:04:35.176Z DEBUG zap/logger.go:31 Histogram value recorded {"metric": "command.duration.command_CreateNode", "value": 60} |
| 2025-09-03 06:04:35.176 | 2025-09-03T06:04:35.176Z INFO zap/logger.go:36 Command completed {"command": "CreateNode", "duration": "60.382651ms"} |
| 2025-09-03 06:04:35.176 | 2025-09-03T06:04:35.176Z DEBUG zap/logger.go:31 Counter incremented {"metric": "command.success.command_CreateNode", "value": 6} |
| 2025-09-03 06:04:35.176 | 2025-09-03T06:04:35.176Z DEBUG zap/logger.go:31 Histogram value recorded {"metric": "command.duration.command_CreateNode", "value": 60} |
| 2025-09-03 06:04:35.126 | 2025-09-03T06:04:35.125Z DEBUG zap/logger.go:31 Node saved successfully {"node_id": "7e2ac84b-8ae5-43c6-baf1-d057f07279f4", "version": 1} |
| 2025-09-03 06:04:35.126 | 2025-09-03T06:04:35.126Z DEBUG zap/logger.go:31 No items to commit |
| 2025-09-03 06:04:35.121 | 2025-09-03T06:04:35.121Z DEBUG zap/logger.go:31 Events saved successfully {"aggregate_id": "7e2ac84b-8ae5-43c6-baf1-d057f07279f4", "event_count": 1, "expected_version": 0} |
| 2025-09-03 06:04:35.116 | 2025/09/03 06:04:35 Processing POST-COLD-START request (14.335328886s after cold start): POST /api/v1/nodes |
| 2025-09-03 06:04:35.116 | 2025-09-03T06:04:35.116Z INFO zap/logger.go:36 Executing command {"command": "CreateNode", "correlation_id": "622cb56d-e617-4269-90c7-21289ee3487d"} |
| 2025-09-03 06:04:35.116 | 2025-09-03T06:04:35.116Z DEBUG zap/logger.go:31 Unit of work started |
| 2025-09-03 06:04:35.115 | START RequestId: 51bc105a-c4d5-43bc-8130-6ac18fa12f0f Version: $LATEST |
| 2025-09-03 06:04:30.716 | END RequestId: 61ffab71-1616-48ea-b576-b0fb3a54ae4d |
| 2025-09-03 06:04:30.716 | REPORT RequestId: 61ffab71-1616-48ea-b576-b0fb3a54ae4d Duration: 37.35 ms Billed Duration: 38 ms Memory Size: 128 MB Max Memory Used: 49 MB |
| 2025-09-03 06:04:30.707 | END RequestId: 74e77b68-db02-43b4-9c04-080c773b35af |
| 2025-09-03 06:04:30.707 | REPORT RequestId: 74e77b68-db02-43b4-9c04-080c773b35af Duration: 29.33 ms Billed Duration: 30 ms Memory Size: 128 MB Max Memory Used: 49 MB |
| 2025-09-03 06:04:30.699 | 2025/09/03 06:04:30 [169.254.30.153/cOHUgATgGc-000009] "GET http://f51uq07z1g.execute-api.us-west-2.amazonaws.com/api/v1/categories HTTP/1.1" from 73.140.251.80 - 200 18B in 19.944082ms |
| 2025-09-03 06:04:30.699 | 2025/09/03 06:04:30 Post-cold-start request completed in 20.040489ms: GET /api/v1/categories -> 200 (cold start age: 9.899120667s) |
| 2025-09-03 06:04:30.689 | END RequestId: 87e421df-4da4-46f9-994d-b22f4182aa59 |
| 2025-09-03 06:04:30.689 | REPORT RequestId: 87e421df-4da4-46f9-994d-b22f4182aa59 Duration: 10.77 ms Billed Duration: 11 ms Memory Size: 128 MB Max Memory Used: 50 MB |
| 2025-09-03 06:04:30.687 | 2025-09-03T06:04:30.687Z DEBUG zap/logger.go:31 Retrieved graph {"user_id": "125deabf-b32e-4313-b893-4a3ddb416cc2", "node_id": "", "nodes": 3, "edges": 0} |
| 2025-09-03 06:04:30.687 | 2025-09-03T06:04:30.687Z DEBUG zap/logger.go:31 Counter incremented {"metric": "query.success.query_GetGraph", "value": 5} |
| 2025-09-03 06:04:30.687 | 2025-09-03T06:04:30.687Z DEBUG zap/logger.go:31 Histogram value recorded {"metric": "query.duration.query_GetGraph", "value": 8} |
| 2025-09-03 06:04:30.687 | 2025-09-03T06:04:30.687Z DEBUG zap/logger.go:31 Query completed {"query": "GetGraph", "duration": "8.649185ms"} |
| 2025-09-03 06:04:30.687 | 2025-09-03T06:04:30.687Z DEBUG zap/logger.go:31 Counter incremented {"metric": "query.success.query_GetGraph", "value": 6} |
| 2025-09-03 06:04:30.687 | 2025-09-03T06:04:30.687Z DEBUG zap/logger.go:31 Histogram value recorded {"metric": "query.duration.query_GetGraph", "value": 8} |
| 2025-09-03 06:04:30.687 | 2025/09/03 06:04:30 DEBUG: GetGraphData succeeded, graph has 3 nodes and 0 edges |
| 2025-09-03 06:04:30.687 | 2025/09/03 06:04:30 DEBUG: GetGraphData completed successfully - returning 3 elements |
| 2025-09-03 06:04:30.687 | 2025/09/03 06:04:30 [169.254.6.221/80i4fWdDpR-000003] "GET http://f51uq07z1g.execute-api.us-west-2.amazonaws.com/api/v1/graph-data HTTP/1.1" from 73.140.251.80 - 200 308B in 8.762516ms |
| 2025-09-03 06:04:30.687 | 2025/09/03 06:04:30 Post-cold-start request completed in 8.836835ms: GET /api/v1/graph-data -> 200 (cold start age: 9.914056272s) |
| 2025-09-03 06:04:30.681 | 2025-09-03T06:04:30.681Z DEBUG zap/logger.go:31 Retrieved nodes for user {"user_id": "125deabf-b32e-4313-b893-4a3ddb416cc2", "count": 3, "total": 3} |
| 2025-09-03 06:04:30.681 | 2025-09-03T06:04:30.681Z DEBUG zap/logger.go:31 Counter incremented {"metric": "query.success.query_GetNodesByUser", "value": 1} |
| 2025-09-03 06:04:30.681 | 2025-09-03T06:04:30.681Z DEBUG zap/logger.go:31 Histogram value recorded {"metric": "query.duration.query_GetNodesByUser", "value": 2} |
| 2025-09-03 06:04:30.681 | 2025-09-03T06:04:30.681Z DEBUG zap/logger.go:31 Query completed {"query": "GetNodesByUser", "duration": "2.674924ms"} |
| 2025-09-03 06:04:30.681 | 2025-09-03T06:04:30.681Z DEBUG zap/logger.go:31 Counter incremented {"metric": "query.success.query_GetNodesByUser", "value": 2} |
| 2025-09-03 06:04:30.681 | 2025-09-03T06:04:30.681Z DEBUG zap/logger.go:31 Histogram value recorded {"metric": "query.duration.query_GetNodesByUser", "value": 2} |
| 2025-09-03 06:04:30.681 | 2025/09/03 06:04:30 [169.254.4.101/xD3s5fXJwl-000006] "GET http://f51uq07z1g.execute-api.us-west-2.amazonaws.com/api/v1/nodes?limit=50 HTTP/1.1" from 73.140.251.80 - 200 751B in 2.927157ms |
| 2025-09-03 06:04:30.681 | 2025/09/03 06:04:30 Post-cold-start request completed in 3.024882ms: GET /api/v1/nodes -> 200 (cold start age: 9.960697896s) |
| 2025-09-03 06:04:30.679 | 2025-09-03T06:04:30.678Z DEBUG zap/logger.go:31 Executing query {"query": "GetGraph"} |
| 2025-09-03 06:04:30.679 | START RequestId: 61ffab71-1616-48ea-b576-b0fb3a54ae4d Version: $LATEST |
| 2025-09-03 06:04:30.679 | 2025/09/03 06:04:30 Processing POST-COLD-START request (9.899120667s after cold start): GET /api/v1/categories |
| 2025-09-03 06:04:30.679 | 2025/09/03 06:04:30 DEBUG: ListCategories called for userID: 125deabf-b32e-4313-b893-4a3ddb416cc2 |
| 2025-09-03 06:04:30.678 | START RequestId: 74e77b68-db02-43b4-9c04-080c773b35af Version: $LATEST |
| 2025-09-03 06:04:30.678 | 2025/09/03 06:04:30 Processing POST-COLD-START request (9.914056272s after cold start): GET /api/v1/graph-data |
| 2025-09-03 06:04:30.678 | 2025/09/03 06:04:30 DEBUG: GetGraphData called for userID: 125deabf-b32e-4313-b893-4a3ddb416cc2 |
| 2025-09-03 06:04:30.678 | 2025/09/03 06:04:30 DEBUG: Calling queryBus.Send with GetGraphDataQuery |
| 2025-09-03 06:04:30.678 | START RequestId: 87e421df-4da4-46f9-994d-b22f4182aa59 Version: $LATEST |
| 2025-09-03 06:04:30.678 | 2025/09/03 06:04:30 Processing POST-COLD-START request (9.960697896s after cold start): GET /api/v1/nodes |
| 2025-09-03 06:04:30.678 | 2025-09-03T06:04:30.678Z DEBUG zap/logger.go:31 Executing query {"query": "GetNodesByUser"} |
| 2025-09-03 06:04:30.677 | 2025/09/03 06:04:30 Successfully processed node , created 0 edges from 0 candidates |
| 2025-09-03 06:04:30.677 | END RequestId: 6c3fc872-7791-4ba5-87c2-6b2889711d8d |
| 2025-09-03 06:04:30.677 | REPORT RequestId: 6c3fc872-7791-4ba5-87c2-6b2889711d8d Duration: 5.37 ms Billed Duration: 6 ms Memory Size: 128 MB Max Memory Used: 37 MB |
| 2025-09-03 06:04:30.672 | START RequestId: 6c3fc872-7791-4ba5-87c2-6b2889711d8d Version: $LATEST |
| 2025-09-03 06:04:30.672 | 2025/09/03 06:04:30 Processing NodeCreated event for node  with 0 keywords |
| 2025-09-03 06:04:30.629 | END RequestId: 6c0af26d-9790-4bdb-b45f-7551e759f5c5 |
| 2025-09-03 06:04:30.629 | REPORT RequestId: 6c0af26d-9790-4bdb-b45f-7551e759f5c5 Duration: 4.69 ms Billed Duration: 5 ms Memory Size: 128 MB Max Memory Used: 37 MB |
| 2025-09-03 06:04:30.629 | END RequestId: e29254fc-cb13-420c-8bb8-b1310892c87a |
| 2025-09-03 06:04:30.629 | REPORT RequestId: e29254fc-cb13-420c-8bb8-b1310892c87a Duration: 8.27 ms Billed Duration: 9 ms Memory Size: 128 MB Max Memory Used: 50 MB |
| 2025-09-03 06:04:30.628 | 2025/09/03 06:04:30 Successfully processed node , created 0 edges from 0 candidates |
| 2025-09-03 06:04:30.628 | 2025/09/03 06:04:30 DEBUG: CategorizeNode called for userID: 125deabf-b32e-4313-b893-4a3ddb416cc2 |
| 2025-09-03 06:04:30.628 | 2025/09/03 06:04:30 [169.254.4.101/xD3s5fXJwl-000005] "POST http://f51uq07z1g.execute-api.us-west-2.amazonaws.com/api/v1/nodes/ee7b610e6b239678/categories HTTP/1.1" from 73.140.251.80 - 200 98B in 89.425µs |
| 2025-09-03 06:04:30.628 | 2025/09/03 06:04:30 Post-cold-start request completed in 6.772843ms: POST /api/v1/nodes/ee7b610e6b239678/categories -> 200 (cold start age: 9.90420402s) |
| 2025-09-03 06:04:30.625 | 2025/09/03 06:04:30 Processing NodeCreated event for node  with 0 keywords |
| 2025-09-03 06:04:30.624 | START RequestId: 6c0af26d-9790-4bdb-b45f-7551e759f5c5 Version: $LATEST |
| 2025-09-03 06:04:30.622 | 2025/09/03 06:04:30 Processing POST-COLD-START request (9.90420402s after cold start): POST /api/v1/nodes/ee7b610e6b239678/categories |
| 2025-09-03 06:04:30.621 | START RequestId: e29254fc-cb13-420c-8bb8-b1310892c87a Version: $LATEST |
| 2025-09-03 06:04:30.550 | END RequestId: 104d24d8-777c-4bd5-9cd5-5d651ff75a31 |
| 2025-09-03 06:04:30.550 | REPORT RequestId: 104d24d8-777c-4bd5-9cd5-5d651ff75a31 Duration: 259.58 ms Billed Duration: 260 ms Memory Size: 128 MB Max Memory Used: 50 MB |
| 2025-09-03 06:04:30.549 | 2025/09/03 06:04:30 [169.254.4.101/xD3s5fXJwl-000004] "POST http://f51uq07z1g.execute-api.us-west-2.amazonaws.com/api/v1/nodes HTTP/1.1" from 73.140.251.80 - 201 112B in 257.746994ms |
| 2025-09-03 06:04:30.549 | 2025/09/03 06:04:30 Post-cold-start request completed in 257.940487ms: POST /api/v1/nodes -> 201 (cold start age: 9.573310314s) |
| 2025-09-03 06:04:30.530 | 2025-09-03T06:04:30.529Z DEBUG zap/logger.go:31 Counter incremented {"metric": "node.created.user_id_125deabf-b32e-4313-b893-4a3ddb416cc2.has_tags_false", "value": 1} |
| 2025-09-03 06:04:30.530 | 2025-09-03T06:04:30.529Z INFO zap/logger.go:36 Node created successfully {"node_id": "5aa3da34-16e3-46a7-8309-660dcb3239e5", "user_id": "125deabf-b32e-4313-b893-4a3ddb416cc2"} |
| 2025-09-03 06:04:30.530 | 2025-09-03T06:04:30.529Z DEBUG zap/logger.go:31 Counter incremented {"metric": "command.success.command_CreateNode", "value": 1} |
| 2025-09-03 06:04:30.530 | 2025-09-03T06:04:30.529Z DEBUG zap/logger.go:31 Histogram value recorded {"metric": "command.duration.command_CreateNode", "value": 237} |
| 2025-09-03 06:04:30.530 | 2025-09-03T06:04:30.529Z INFO zap/logger.go:36 Command completed {"command": "CreateNode", "duration": "237.929038ms"} |
| 2025-09-03 06:04:30.530 | 2025-09-03T06:04:30.529Z DEBUG zap/logger.go:31 Counter incremented {"metric": "command.success.command_CreateNode", "value": 2} |
| 2025-09-03 06:04:30.530 | 2025-09-03T06:04:30.529Z DEBUG zap/logger.go:31 Histogram value recorded {"metric": "command.duration.command_CreateNode", "value": 237} |
| 2025-09-03 06:04:30.329 | 2025-09-03T06:04:30.329Z DEBUG zap/logger.go:31 Node saved successfully {"node_id": "5aa3da34-16e3-46a7-8309-660dcb3239e5", "version": 1} |
| 2025-09-03 06:04:30.329 | 2025-09-03T06:04:30.329Z DEBUG zap/logger.go:31 No items to commit |
| 2025-09-03 06:04:30.309 | 2025-09-03T06:04:30.309Z DEBUG zap/logger.go:31 Events saved successfully {"aggregate_id": "5aa3da34-16e3-46a7-8309-660dcb3239e5", "event_count": 1, "expected_version": 0} |
| 2025-09-03 06:04:30.291 | 2025/09/03 06:04:30 Processing POST-COLD-START request (9.573310314s after cold start): POST /api/v1/nodes |
| 2025-09-03 06:04:30.291 | 2025-09-03T06:04:30.291Z INFO zap/logger.go:36 Executing command {"command": "CreateNode", "correlation_id": "1c6bfce4-5187-4cba-84e5-031f785c070a"} |
| 2025-09-03 06:04:30.291 | 2025-09-03T06:04:30.291Z DEBUG zap/logger.go:31 Unit of work started |
| 2025-09-03 06:04:30.290 | START RequestId: 104d24d8-777c-4bd5-9cd5-5d651ff75a31 Version: $LATEST |
| 2025-09-03 06:04:25.149 | END RequestId: 0f3bad96-3b6b-4004-9cb3-a4e813dbdc40 |
| 2025-09-03 06:04:25.149 | REPORT RequestId: 0f3bad96-3b6b-4004-9cb3-a4e813dbdc40 Duration: 12.10 ms Billed Duration: 13 ms Memory Size: 128 MB Max Memory Used: 49 MB |
| 2025-09-03 06:04:25.146 | 2025-09-03T06:04:25.145Z DEBUG zap/logger.go:31 Histogram value recorded {"metric": "query.duration.query_GetGraph", "value": 2} |
| 2025-09-03 06:04:25.146 | 2025-09-03T06:04:25.146Z DEBUG zap/logger.go:31 Query completed {"query": "GetGraph", "duration": "2.542733ms"} |
| 2025-09-03 06:04:25.146 | 2025-09-03T06:04:25.146Z DEBUG zap/logger.go:31 Counter incremented {"metric": "query.success.query_GetGraph", "value": 4} |
| 2025-09-03 06:04:25.146 | 2025-09-03T06:04:25.146Z DEBUG zap/logger.go:31 Histogram value recorded {"metric": "query.duration.query_GetGraph", "value": 2} |
| 2025-09-03 06:04:25.146 | 2025/09/03 06:04:25 DEBUG: GetGraphData succeeded, graph has 2 nodes and 0 edges |
| 2025-09-03 06:04:25.146 | 2025/09/03 06:04:25 DEBUG: GetGraphData completed successfully - returning 2 elements |
| 2025-09-03 06:04:25.146 | 2025/09/03 06:04:25 [169.254.6.221/80i4fWdDpR-000002] "GET http://f51uq07z1g.execute-api.us-west-2.amazonaws.com/api/v1/graph-data HTTP/1.1" from 73.140.251.80 - 200 204B in 2.689813ms |
| 2025-09-03 06:04:25.146 | 2025/09/03 06:04:25 Post-cold-start request completed in 2.779706ms: GET /api/v1/graph-data -> 200 (cold start age: 4.378520217s) |
| 2025-09-03 06:04:25.146 | END RequestId: 61e417d7-c8e4-4d43-8294-52ac8ca73e7b |
| 2025-09-03 06:04:25.146 | REPORT RequestId: 61e417d7-c8e4-4d43-8294-52ac8ca73e7b Duration: 3.96 ms Billed Duration: 4 ms Memory Size: 128 MB Max Memory Used: 49 MB |
| 2025-09-03 06:04:25.145 | 2025-09-03T06:04:25.145Z DEBUG zap/logger.go:31 Retrieved graph {"user_id": "125deabf-b32e-4313-b893-4a3ddb416cc2", "node_id": "", "nodes": 2, "edges": 0} |
| 2025-09-03 06:04:25.145 | 2025-09-03T06:04:25.145Z DEBUG zap/logger.go:31 Counter incremented {"metric": "query.success.query_GetGraph", "value": 3} |
| 2025-09-03 06:04:25.143 | 2025/09/03 06:04:25 Processing POST-COLD-START request (4.378520217s after cold start): GET /api/v1/graph-data |
| 2025-09-03 06:04:25.143 | 2025/09/03 06:04:25 DEBUG: GetGraphData called for userID: 125deabf-b32e-4313-b893-4a3ddb416cc2 |
| 2025-09-03 06:04:25.143 | 2025/09/03 06:04:25 DEBUG: Calling queryBus.Send with GetGraphDataQuery |
| 2025-09-03 06:04:25.143 | 2025-09-03T06:04:25.143Z DEBUG zap/logger.go:31 Executing query {"query": "GetGraph"} |
| 2025-09-03 06:04:25.143 | 2025/09/03 06:04:25 [169.254.4.101/xD3s5fXJwl-000003] "GET http://f51uq07z1g.execute-api.us-west-2.amazonaws.com/api/v1/categories HTTP/1.1" from 73.140.251.80 - 200 18B in 5.042855ms |
| 2025-09-03 06:04:25.143 | 2025/09/03 06:04:25 Post-cold-start request completed in 5.135383ms: GET /api/v1/categories -> 200 (cold start age: 4.419979474s) |
| 2025-09-03 06:04:25.142 | START RequestId: 61e417d7-c8e4-4d43-8294-52ac8ca73e7b Version: $LATEST |
| 2025-09-03 06:04:25.141 | END RequestId: d7a8b7ea-54a0-48de-8d9e-1f2dbfe2f722 |
| 2025-09-03 06:04:25.141 | REPORT RequestId: d7a8b7ea-54a0-48de-8d9e-1f2dbfe2f722 Duration: 13.81 ms Billed Duration: 14 ms Memory Size: 128 MB Max Memory Used: 37 MB |
| 2025-09-03 06:04:25.140 | 2025/09/03 06:04:25 Successfully processed node , created 0 edges from 0 candidates |
| 2025-09-03 06:04:25.140 | END RequestId: c2506d8e-7214-4a17-8413-2f43ff1f4a60 |
| 2025-09-03 06:04:25.140 | REPORT RequestId: c2506d8e-7214-4a17-8413-2f43ff1f4a60 Duration: 6.68 ms Billed Duration: 7 ms Memory Size: 128 MB Max Memory Used: 36 MB |
| 2025-09-03 06:04:25.138 | 2025/09/03 06:04:25 DEBUG: ListCategories called for userID: 125deabf-b32e-4313-b893-4a3ddb416cc2 |
| 2025-09-03 06:04:25.137 | END RequestId: 0857eaca-13f9-4b3e-8e56-a1e0a686c4d7 |
| 2025-09-03 06:04:25.137 | REPORT RequestId: 0857eaca-13f9-4b3e-8e56-a1e0a686c4d7 Duration: 6.70 ms Billed Duration: 7 ms Memory Size: 128 MB Max Memory Used: 48 MB |
| 2025-09-03 06:04:25.137 | 2025/09/03 06:04:25 Successfully processed node , created 0 edges from 0 candidates |
| 2025-09-03 06:04:25.137 | START RequestId: 0f3bad96-3b6b-4004-9cb3-a4e813dbdc40 Version: $LATEST |
| 2025-09-03 06:04:25.137 | 2025/09/03 06:04:25 Processing POST-COLD-START request (4.419979474s after cold start): GET /api/v1/categories |
| 2025-09-03 06:04:25.136 | 2025-09-03T06:04:25.134Z DEBUG zap/logger.go:31 Retrieved nodes for user {"user_id": "125deabf-b32e-4313-b893-4a3ddb416cc2", "count": 2, "total": 2} |
| 2025-09-03 06:04:25.136 | 2025-09-03T06:04:25.134Z DEBUG zap/logger.go:31 Counter incremented {"metric": "query.success.query_GetNodesByUser", "value": 5} |
| 2025-09-03 06:04:25.136 | 2025-09-03T06:04:25.134Z DEBUG zap/logger.go:31 Histogram value recorded {"metric": "query.duration.query_GetNodesByUser", "value": 3} |
| 2025-09-03 06:04:25.136 | 2025-09-03T06:04:25.134Z DEBUG zap/logger.go:31 Query completed {"query": "GetNodesByUser", "duration": "3.523931ms"} |
| 2025-09-03 06:04:25.136 | 2025-09-03T06:04:25.134Z DEBUG zap/logger.go:31 Counter incremented {"metric": "query.success.query_GetNodesByUser", "value": 6} |
| 2025-09-03 06:04:25.136 | 2025-09-03T06:04:25.134Z DEBUG zap/logger.go:31 Histogram value recorded {"metric": "query.duration.query_GetNodesByUser", "value": 3} |
| 2025-09-03 06:04:25.136 | 2025/09/03 06:04:25 [169.254.30.153/cOHUgATgGc-000008] "GET http://f51uq07z1g.execute-api.us-west-2.amazonaws.com/api/v1/nodes?limit=50 HTTP/1.1" from 73.140.251.80 - 200 512B in 3.652775ms |
| 2025-09-03 06:04:25.136 | 2025/09/03 06:04:25 Post-cold-start request completed in 3.765505ms: GET /api/v1/nodes -> 200 (cold start age: 4.349987864s) |
| 2025-09-03 06:04:25.134 | START RequestId: c2506d8e-7214-4a17-8413-2f43ff1f4a60 Version: $LATEST |
| 2025-09-03 06:04:25.134 | 2025/09/03 06:04:25 Processing NodeCreated event for node  with 0 keywords |
| 2025-09-03 06:04:25.130 | START RequestId: 0857eaca-13f9-4b3e-8e56-a1e0a686c4d7 Version: $LATEST |
| 2025-09-03 06:04:25.130 | 2025/09/03 06:04:25 Processing POST-COLD-START request (4.349987864s after cold start): GET /api/v1/nodes |
| 2025-09-03 06:04:25.130 | 2025-09-03T06:04:25.130Z DEBUG zap/logger.go:31 Executing query {"query": "GetNodesByUser"} |
| 2025-09-03 06:04:25.127 | START RequestId: d7a8b7ea-54a0-48de-8d9e-1f2dbfe2f722 Version: $LATEST |
| 2025-09-03 06:04:25.127 | 2025/09/03 06:04:25 Processing NodeCreated event for node  with 0 keywords |
| 2025-09-03 06:04:25.090 | 2025/09/03 06:04:25 Processing POST-COLD-START request (4.309274925s after cold start): POST /api/v1/nodes/e8344ebcf71f9f98/categories |
| 2025-09-03 06:04:25.090 | 2025/09/03 06:04:25 DEBUG: CategorizeNode called for userID: 125deabf-b32e-4313-b893-4a3ddb416cc2 |
| 2025-09-03 06:04:25.090 | 2025/09/03 06:04:25 [169.254.30.153/cOHUgATgGc-000007] "POST http://f51uq07z1g.execute-api.us-west-2.amazonaws.com/api/v1/nodes/e8344ebcf71f9f98/categories HTTP/1.1" from 73.140.251.80 - 200 98B in 30.065µs |
| 2025-09-03 06:04:25.090 | 2025/09/03 06:04:25 Post-cold-start request completed in 117.717µs: POST /api/v1/nodes/e8344ebcf71f9f98/categories -> 200 (cold start age: 4.309274925s) |
| 2025-09-03 06:04:25.090 | END RequestId: e66380d0-abc6-4da0-b3e4-74fd754c373d |
| 2025-09-03 06:04:25.090 | REPORT RequestId: e66380d0-abc6-4da0-b3e4-74fd754c373d Duration: 1.23 ms Billed Duration: 2 ms Memory Size: 128 MB Max Memory Used: 48 MB |
| 2025-09-03 06:04:25.089 | START RequestId: e66380d0-abc6-4da0-b3e4-74fd754c373d Version: $LATEST |
| 2025-09-03 06:04:25.017 | END RequestId: d2727bfe-a474-4355-8cf3-60b6568fee3f |
| 2025-09-03 06:04:25.017 | REPORT RequestId: d2727bfe-a474-4355-8cf3-60b6568fee3f Duration: 50.81 ms Billed Duration: 51 ms Memory Size: 128 MB Max Memory Used: 48 MB |
| 2025-09-03 06:04:25.000 | 2025/09/03 06:04:25 [169.254.30.153/cOHUgATgGc-000006] "POST http://f51uq07z1g.execute-api.us-west-2.amazonaws.com/api/v1/nodes HTTP/1.1" from 73.140.251.80 - 201 103B in 33.2026ms |
| 2025-09-03 06:04:25.000 | 2025/09/03 06:04:25 Post-cold-start request completed in 33.294197ms: POST /api/v1/nodes -> 201 (cold start age: 4.186022133s) |
| 2025-09-03 06:04:24.997 | 2025-09-03T06:04:24.997Z DEBUG zap/logger.go:31 Counter incremented {"metric": "node.created.user_id_125deabf-b32e-4313-b893-4a3ddb416cc2.has_tags_false", "value": 2} |
| 2025-09-03 06:04:24.997 | 2025-09-03T06:04:24.997Z INFO zap/logger.go:36 Node created successfully {"node_id": "77b8fb00-4721-4165-8ee1-01c25de3ee84", "user_id": "125deabf-b32e-4313-b893-4a3ddb416cc2"} |
| 2025-09-03 06:04:24.997 | 2025-09-03T06:04:24.997Z DEBUG zap/logger.go:31 Counter incremented {"metric": "command.success.command_CreateNode", "value": 3} |
| 2025-09-03 06:04:24.997 | 2025-09-03T06:04:24.997Z DEBUG zap/logger.go:31 Histogram value recorded {"metric": "command.duration.command_CreateNode", "value": 30} |
| 2025-09-03 06:04:24.997 | 2025-09-03T06:04:24.997Z INFO zap/logger.go:36 Command completed {"command": "CreateNode", "duration": "30.256039ms"} |
| 2025-09-03 06:04:24.997 | 2025-09-03T06:04:24.997Z DEBUG zap/logger.go:31 Counter incremented {"metric": "command.success.command_CreateNode", "value": 4} |
| 2025-09-03 06:04:24.997 | 2025-09-03T06:04:24.997Z DEBUG zap/logger.go:31 Histogram value recorded {"metric": "command.duration.command_CreateNode", "value": 30} |
| 2025-09-03 06:04:24.976 | 2025-09-03T06:04:24.976Z DEBUG zap/logger.go:31 Node saved successfully {"node_id": "77b8fb00-4721-4165-8ee1-01c25de3ee84", "version": 1} |
| 2025-09-03 06:04:24.976 | 2025-09-03T06:04:24.976Z DEBUG zap/logger.go:31 No items to commit |
| 2025-09-03 06:04:24.971 | 2025-09-03T06:04:24.971Z DEBUG zap/logger.go:31 Events saved successfully {"aggregate_id": "77b8fb00-4721-4165-8ee1-01c25de3ee84", "event_count": 1, "expected_version": 0} |
| 2025-09-03 06:04:24.966 | START RequestId: d2727bfe-a474-4355-8cf3-60b6568fee3f Version: $LATEST |
| 2025-09-03 06:04:24.966 | 2025/09/03 06:04:24 Processing POST-COLD-START request (4.186022133s after cold start): POST /api/v1/nodes |
| 2025-09-03 06:04:24.966 | 2025-09-03T06:04:24.966Z INFO zap/logger.go:36 Executing command {"command": "CreateNode", "correlation_id": "a3c5059b-6156-4dbb-b44e-48435c012b70"} |
| 2025-09-03 06:04:24.966 | 2025-09-03T06:04:24.966Z DEBUG zap/logger.go:31 Unit of work started |
| 2025-09-03 06:04:23.941 | END RequestId: f8924a08-dae7-4876-b12e-121ae465901b |
| 2025-09-03 06:04:23.941 | REPORT RequestId: f8924a08-dae7-4876-b12e-121ae465901b Duration: 698.64 ms Billed Duration: 836 ms Memory Size: 128 MB Max Memory Used: 36 MB Init Duration: 137.26 ms |
| 2025-09-03 06:04:23.940 | 2025/09/03 06:04:23 Successfully processed node , created 0 edges from 0 candidates |
| 2025-09-03 06:04:23.902 | END RequestId: dc467a9e-03bd-44ac-887c-46141a2ca2fd |
| 2025-09-03 06:04:23.902 | REPORT RequestId: dc467a9e-03bd-44ac-887c-46141a2ca2fd Duration: 679.30 ms Billed Duration: 817 ms Memory Size: 128 MB Max Memory Used: 35 MB Init Duration: 137.18 ms |
| 2025-09-03 06:04:23.881 | 2025/09/03 06:04:23 Successfully processed node , created 0 edges from 0 candidates |
| 2025-09-03 06:04:23.243 | START RequestId: f8924a08-dae7-4876-b12e-121ae465901b Version: $LATEST |
| 2025-09-03 06:04:23.243 | 2025/09/03 06:04:23 Processing NodeCreated event for node  with 0 keywords |
| 2025-09-03 06:04:23.223 | 2025/09/03 06:04:23 Processing NodeCreated event for node  with 0 keywords |
| 2025-09-03 06:04:23.222 | START RequestId: dc467a9e-03bd-44ac-887c-46141a2ca2fd Version: $LATEST |
| 2025-09-03 06:04:23.104 | INIT_START Runtime Version: provided:al2.v126 Runtime Version ARN: arn:aws:lambda:us-west-2::runtime:19258f80605e632e71fce9f74eb7eef4421eb4e2c946c551c7eb27c55b4b3c63 |
| 2025-09-03 06:04:23.084 | INIT_START Runtime Version: provided:al2.v126 Runtime Version ARN: arn:aws:lambda:us-west-2::runtime:19258f80605e632e71fce9f74eb7eef4421eb4e2c946c551c7eb27c55b4b3c63 |
| 2025-09-03 06:04:22.940 | END RequestId: 0d614158-7023-4675-8a91-96ece5cf188c |
| 2025-09-03 06:04:22.940 | REPORT RequestId: 0d614158-7023-4675-8a91-96ece5cf188c Duration: 9.20 ms Billed Duration: 10 ms Memory Size: 128 MB Max Memory Used: 48 MB |
| 2025-09-03 06:04:22.939 | 2025/09/03 06:04:22 [169.254.30.153/cOHUgATgGc-000005] "GET http://f51uq07z1g.execute-api.us-west-2.amazonaws.com/api/v1/categories HTTP/1.1" from 73.140.251.80 - 200 18B in 7.915199ms |
| 2025-09-03 06:04:22.939 | 2025/09/03 06:04:22 Post-cold-start request completed in 8.000268ms: GET /api/v1/categories -> 200 (cold start age: 2.150571272s) |
| 2025-09-03 06:04:22.931 | START RequestId: 0d614158-7023-4675-8a91-96ece5cf188c Version: $LATEST |
| 2025-09-03 06:04:22.931 | 2025/09/03 06:04:22 Processing POST-COLD-START request (2.150571272s after cold start): GET /api/v1/categories |
| 2025-09-03 06:04:22.931 | 2025/09/03 06:04:22 DEBUG: ListCategories called for userID: 125deabf-b32e-4313-b893-4a3ddb416cc2 |
| 2025-09-03 06:04:22.929 | END RequestId: 3d769c98-f647-4faf-a2e9-649ed3c21ca0 |
| 2025-09-03 06:04:22.929 | REPORT RequestId: 3d769c98-f647-4faf-a2e9-649ed3c21ca0 Duration: 17.89 ms Billed Duration: 18 ms Memory Size: 128 MB Max Memory Used: 49 MB |
| 2025-09-03 06:04:22.917 | END RequestId: fe253e81-1a20-402c-abff-b84736e79f92 |
| 2025-09-03 06:04:22.917 | REPORT RequestId: fe253e81-1a20-402c-abff-b84736e79f92 Duration: 9.57 ms Billed Duration: 10 ms Memory Size: 128 MB Max Memory Used: 48 MB |
| 2025-09-03 06:04:22.916 | 2025-09-03T06:04:22.916Z DEBUG zap/logger.go:31 Counter incremented {"metric": "query.success.query_GetNodesByUser", "value": 3} |
| 2025-09-03 06:04:22.916 | 2025-09-03T06:04:22.916Z DEBUG zap/logger.go:31 Histogram value recorded {"metric": "query.duration.query_GetNodesByUser", "value": 8} |
| 2025-09-03 06:04:22.916 | 2025-09-03T06:04:22.916Z DEBUG zap/logger.go:31 Query completed {"query": "GetNodesByUser", "duration": "8.073907ms"} |
| 2025-09-03 06:04:22.916 | 2025-09-03T06:04:22.916Z DEBUG zap/logger.go:31 Counter incremented {"metric": "query.success.query_GetNodesByUser", "value": 4} |
| 2025-09-03 06:04:22.916 | 2025-09-03T06:04:22.916Z DEBUG zap/logger.go:31 Histogram value recorded {"metric": "query.duration.query_GetNodesByUser", "value": 8} |
| 2025-09-03 06:04:22.916 | 2025/09/03 06:04:22 [169.254.30.153/cOHUgATgGc-000004] "GET http://f51uq07z1g.execute-api.us-west-2.amazonaws.com/api/v1/nodes?limit=50 HTTP/1.1" from 73.140.251.80 - 200 282B in 8.221266ms |
| 2025-09-03 06:04:22.916 | 2025/09/03 06:04:22 Post-cold-start request completed in 8.311297ms: GET /api/v1/nodes -> 200 (cold start age: 2.127311933s) |
| 2025-09-03 06:04:22.915 | 2025-09-03T06:04:22.915Z DEBUG zap/logger.go:31 Retrieved graph {"user_id": "125deabf-b32e-4313-b893-4a3ddb416cc2", "node_id": "", "nodes": 1, "edges": 0} |
| 2025-09-03 06:04:22.915 | 2025-09-03T06:04:22.915Z DEBUG zap/logger.go:31 Counter incremented {"metric": "query.success.query_GetGraph", "value": 1} |
| 2025-09-03 06:04:22.915 | 2025-09-03T06:04:22.915Z DEBUG zap/logger.go:31 Histogram value recorded {"metric": "query.duration.query_GetGraph", "value": 2} |
| 2025-09-03 06:04:22.915 | 2025-09-03T06:04:22.915Z DEBUG zap/logger.go:31 Query completed {"query": "GetGraph", "duration": "2.868495ms"} |
| 2025-09-03 06:04:22.915 | 2025-09-03T06:04:22.915Z DEBUG zap/logger.go:31 Counter incremented {"metric": "query.success.query_GetGraph", "value": 2} |
| 2025-09-03 06:04:22.915 | 2025-09-03T06:04:22.915Z DEBUG zap/logger.go:31 Histogram value recorded {"metric": "query.duration.query_GetGraph", "value": 2} |
| 2025-09-03 06:04:22.915 | 2025/09/03 06:04:22 DEBUG: GetGraphData succeeded, graph has 1 nodes and 0 edges |
| 2025-09-03 06:04:22.915 | 2025/09/03 06:04:22 DEBUG: GetGraphData completed successfully - returning 1 elements |
| 2025-09-03 06:04:22.915 | 2025/09/03 06:04:22 [169.254.4.101/xD3s5fXJwl-000002] "GET http://f51uq07z1g.execute-api.us-west-2.amazonaws.com/api/v1/graph-data HTTP/1.1" from 73.140.251.80 - 200 109B in 3.086446ms |
| 2025-09-03 06:04:22.915 | 2025/09/03 06:04:22 Post-cold-start request completed in 3.256738ms: GET /api/v1/graph-data -> 200 (cold start age: 2.194096757s) |
| 2025-09-03 06:04:22.912 | 2025-09-03T06:04:22.912Z DEBUG zap/logger.go:31 Retrieved nodes for user {"user_id": "125deabf-b32e-4313-b893-4a3ddb416cc2", "count": 1, "total": 1} |
| 2025-09-03 06:04:22.912 | 2025/09/03 06:04:22 Processing POST-COLD-START request (2.194096757s after cold start): GET /api/v1/graph-data |
| 2025-09-03 06:04:22.912 | 2025/09/03 06:04:22 DEBUG: GetGraphData called for userID: 125deabf-b32e-4313-b893-4a3ddb416cc2 |
| 2025-09-03 06:04:22.912 | 2025/09/03 06:04:22 DEBUG: Calling queryBus.Send with GetGraphDataQuery |
| 2025-09-03 06:04:22.912 | 2025-09-03T06:04:22.912Z DEBUG zap/logger.go:31 Executing query {"query": "GetGraph"} |
| 2025-09-03 06:04:22.911 | START RequestId: 3d769c98-f647-4faf-a2e9-649ed3c21ca0 Version: $LATEST |
| 2025-09-03 06:04:22.908 | 2025/09/03 06:04:22 Processing POST-COLD-START request (2.127311933s after cold start): GET /api/v1/nodes |
| 2025-09-03 06:04:22.908 | 2025-09-03T06:04:22.908Z DEBUG zap/logger.go:31 Executing query {"query": "GetNodesByUser"} |
| 2025-09-03 06:04:22.907 | START RequestId: fe253e81-1a20-402c-abff-b84736e79f92 Version: $LATEST |
| 2025-09-03 06:04:22.855 | END RequestId: 8fabebb7-b236-44ea-b884-69e5557f8f34 |
| 2025-09-03 06:04:22.855 | REPORT RequestId: 8fabebb7-b236-44ea-b884-69e5557f8f34 Duration: 1.54 ms Billed Duration: 2 ms Memory Size: 128 MB Max Memory Used: 48 MB |
| 2025-09-03 06:04:22.854 | START RequestId: 8fabebb7-b236-44ea-b884-69e5557f8f34 Version: $LATEST |
| 2025-09-03 06:04:22.854 | 2025/09/03 06:04:22 Processing POST-COLD-START request (2.073777426s after cold start): POST /api/v1/nodes/30263d4b170518be/categories |
| 2025-09-03 06:04:22.854 | 2025/09/03 06:04:22 DEBUG: CategorizeNode called for userID: 125deabf-b32e-4313-b893-4a3ddb416cc2 |
| 2025-09-03 06:04:22.854 | 2025/09/03 06:04:22 [169.254.30.153/cOHUgATgGc-000003] "POST http://f51uq07z1g.execute-api.us-west-2.amazonaws.com/api/v1/nodes/30263d4b170518be/categories HTTP/1.1" from 73.140.251.80 - 200 98B in 32.689µs |
| 2025-09-03 06:04:22.854 | 2025/09/03 06:04:22 Post-cold-start request completed in 111.999µs: POST /api/v1/nodes/30263d4b170518be/categories -> 200 (cold start age: 2.073777426s) |
| 2025-09-03 06:04:22.777 | END RequestId: 1351823a-efbe-4746-81fc-5a4f1c2f7fd1 |
| 2025-09-03 06:04:22.777 | REPORT RequestId: 1351823a-efbe-4746-81fc-5a4f1c2f7fd1 Duration: 200.03 ms Billed Duration: 201 ms Memory Size: 128 MB Max Memory Used: 48 MB |
| 2025-09-03 06:04:22.776 | 2025/09/03 06:04:22 [169.254.30.153/cOHUgATgGc-000002] "POST http://f51uq07z1g.execute-api.us-west-2.amazonaws.com/api/v1/nodes HTTP/1.1" from 73.140.251.80 - 201 102B in 198.486675ms |
| 2025-09-03 06:04:22.776 | 2025/09/03 06:04:22 Post-cold-start request completed in 198.636212ms: POST /api/v1/nodes -> 201 (cold start age: 1.79736772s) |
| 2025-09-03 06:04:22.762 | 2025-09-03T06:04:22.762Z DEBUG zap/logger.go:31 Counter incremented {"metric": "node.created.user_id_125deabf-b32e-4313-b893-4a3ddb416cc2.has_tags_false", "value": 1} |
| 2025-09-03 06:04:22.762 | 2025-09-03T06:04:22.762Z INFO zap/logger.go:36 Node created successfully {"node_id": "09726acd-c288-4e35-86d9-63be777eeacc", "user_id": "125deabf-b32e-4313-b893-4a3ddb416cc2"} |
| 2025-09-03 06:04:22.762 | 2025-09-03T06:04:22.762Z DEBUG zap/logger.go:31 Counter incremented {"metric": "command.success.command_CreateNode", "value": 1} |
| 2025-09-03 06:04:22.762 | 2025-09-03T06:04:22.762Z DEBUG zap/logger.go:31 Histogram value recorded {"metric": "command.duration.command_CreateNode", "value": 183} |
| 2025-09-03 06:04:22.762 | 2025-09-03T06:04:22.762Z INFO zap/logger.go:36 Command completed {"command": "CreateNode", "duration": "184.005452ms"} |
| 2025-09-03 06:04:22.762 | 2025-09-03T06:04:22.762Z DEBUG zap/logger.go:31 Counter incremented {"metric": "command.success.command_CreateNode", "value": 2} |
| 2025-09-03 06:04:22.762 | 2025-09-03T06:04:22.762Z DEBUG zap/logger.go:31 Histogram value recorded {"metric": "command.duration.command_CreateNode", "value": 184} |
| 2025-09-03 06:04:22.602 | 2025-09-03T06:04:22.602Z DEBUG zap/logger.go:31 Node saved successfully {"node_id": "09726acd-c288-4e35-86d9-63be777eeacc", "version": 1} |
| 2025-09-03 06:04:22.602 | 2025-09-03T06:04:22.602Z DEBUG zap/logger.go:31 No items to commit |
| 2025-09-03 06:04:22.584 | 2025-09-03T06:04:22.584Z DEBUG zap/logger.go:31 Events saved successfully {"aggregate_id": "09726acd-c288-4e35-86d9-63be777eeacc", "event_count": 1, "expected_version": 0} |
| 2025-09-03 06:04:22.578 | 2025/09/03 06:04:22 Processing POST-COLD-START request (1.79736772s after cold start): POST /api/v1/nodes |
| 2025-09-03 06:04:22.578 | 2025-09-03T06:04:22.578Z INFO zap/logger.go:36 Executing command {"command": "CreateNode", "correlation_id": "62948c61-0c77-4a69-b022-25aaa7bef6d8"} |
| 2025-09-03 06:04:22.578 | 2025-09-03T06:04:22.578Z DEBUG zap/logger.go:31 Unit of work started |
| 2025-09-03 06:04:22.577 | START RequestId: 1351823a-efbe-4746-81fc-5a4f1c2f7fd1 Version: $LATEST |
| 2025-09-03 06:04:21.618 | END RequestId: ef4c884a-0fd4-4807-ae2c-72817bb9b872 |
| 2025-09-03 06:04:21.618 | REPORT RequestId: ef4c884a-0fd4-4807-ae2c-72817bb9b872 Duration: 819.03 ms Billed Duration: 1031 ms Memory Size: 128 MB Max Memory Used: 47 MB Init Duration: 211.63 ms |
| 2025-09-03 06:04:21.616 | 2025/09/03 06:04:21 [169.254.30.153/cOHUgATgGc-000001] "GET http://f51uq07z1g.execute-api.us-west-2.amazonaws.com/api/v1/nodes?limit=50 HTTP/1.1" from 73.140.251.80 - 200 54B in 816.3277ms |
| 2025-09-03 06:04:21.616 | 2025/09/03 06:04:21 Post-cold-start request completed in 816.505748ms: GET /api/v1/nodes -> 200 (cold start age: 19.167478ms) |
| 2025-09-03 06:04:21.609 | END RequestId: 9d5c0845-9970-49db-8d95-394dd5474479 |
| 2025-09-03 06:04:21.609 | REPORT RequestId: 9d5c0845-9970-49db-8d95-394dd5474479 Duration: 866.92 ms Billed Duration: 1089 ms Memory Size: 128 MB Max Memory Used: 49 MB Init Duration: 221.76 ms |
| 2025-09-03 06:04:21.573 | 2025/09/03 06:04:21 [169.254.4.101/xD3s5fXJwl-000001] "GET http://f51uq07z1g.execute-api.us-west-2.amazonaws.com/api/v1/categories HTTP/1.1" from 73.140.251.80 - 200 18B in 829.701178ms |
| 2025-09-03 06:04:21.573 | 2025/09/03 06:04:21 Post-cold-start request completed in 829.815942ms: GET /api/v1/categories -> 200 (cold start age: 25.68667ms) |
| 2025-09-03 06:04:21.558 | 2025-09-03T06:04:21.558Z DEBUG zap/logger.go:31 Retrieved nodes for user {"user_id": "125deabf-b32e-4313-b893-4a3ddb416cc2", "count": 0, "total": 0} |
| 2025-09-03 06:04:21.558 | 2025-09-03T06:04:21.558Z DEBUG zap/logger.go:31 Counter incremented {"metric": "query.success.query_GetNodesByUser", "value": 1} |
| 2025-09-03 06:04:21.558 | 2025-09-03T06:04:21.558Z DEBUG zap/logger.go:31 Histogram value recorded {"metric": "query.duration.query_GetNodesByUser", "value": 758} |
| 2025-09-03 06:04:21.558 | 2025-09-03T06:04:21.558Z DEBUG zap/logger.go:31 Query completed {"query": "GetNodesByUser", "duration": "758.367536ms"} |
| 2025-09-03 06:04:21.558 | 2025-09-03T06:04:21.558Z DEBUG zap/logger.go:31 Counter incremented {"metric": "query.success.query_GetNodesByUser", "value": 2} |
| 2025-09-03 06:04:21.558 | 2025-09-03T06:04:21.558Z DEBUG zap/logger.go:31 Histogram value recorded {"metric": "query.duration.query_GetNodesByUser", "value": 758} |
| 2025-09-03 06:04:21.490 | END RequestId: ffbfadde-5803-493b-b5dc-99407195920b |
| 2025-09-03 06:04:21.490 | REPORT RequestId: ffbfadde-5803-493b-b5dc-99407195920b Duration: 707.17 ms Billed Duration: 919 ms Memory Size: 128 MB Max Memory Used: 48 MB Init Duration: 211.78 ms |
| 2025-09-03 06:04:21.487 | 2025/09/03 06:04:21 WARN: GetGraphData returned empty graph, returning empty response |
| 2025-09-03 06:04:21.487 | 2025/09/03 06:04:21 [169.254.6.221/80i4fWdDpR-000001] "GET http://f51uq07z1g.execute-api.us-west-2.amazonaws.com/api/v1/graph-data HTTP/1.1" from 73.140.251.80 - 200 18B in 699.824326ms |
| 2025-09-03 06:04:21.487 | 2025/09/03 06:04:21 Post-cold-start request completed in 699.936408ms: GET /api/v1/graph-data -> 200 (cold start age: 22.726051ms) |
| 2025-09-03 06:04:21.468 | 2025-09-03T06:04:21.468Z DEBUG zap/logger.go:31 Retrieved graph {"user_id": "125deabf-b32e-4313-b893-4a3ddb416cc2", "node_id": "", "nodes": 0, "edges": 0} |
| 2025-09-03 06:04:21.468 | 2025-09-03T06:04:21.468Z DEBUG zap/logger.go:31 Counter incremented {"metric": "query.success.query_GetGraph", "value": 1} |
| 2025-09-03 06:04:21.468 | 2025-09-03T06:04:21.468Z DEBUG zap/logger.go:31 Histogram value recorded {"metric": "query.duration.query_GetGraph", "value": 681} |
| 2025-09-03 06:04:21.468 | 2025-09-03T06:04:21.468Z DEBUG zap/logger.go:31 Query completed {"query": "GetGraph", "duration": "681.154387ms"} |
| 2025-09-03 06:04:21.468 | 2025-09-03T06:04:21.468Z DEBUG zap/logger.go:31 Counter incremented {"metric": "query.success.query_GetGraph", "value": 2} |
| 2025-09-03 06:04:21.468 | 2025-09-03T06:04:21.468Z DEBUG zap/logger.go:31 Histogram value recorded {"metric": "query.duration.query_GetGraph", "value": 681} |
| 2025-09-03 06:04:20.800 | 2025-09-03T06:04:20.800Z DEBUG zap/logger.go:31 Executing query {"query": "GetNodesByUser"} |
| 2025-09-03 06:04:20.799 | START RequestId: ef4c884a-0fd4-4807-ae2c-72817bb9b872 Version: $LATEST |
| 2025-09-03 06:04:20.799 | 2025/09/03 06:04:20 Processing POST-COLD-START request (19.167478ms after cold start): GET /api/v1/nodes |
| 2025-09-03 06:04:20.795 | 2025-09-03T06:04:20.794Z INFO zap/logger.go:36 Registered command handler {"command_type": "DisconnectNodes"} |
| 2025-09-03 06:04:20.795 | 2025-09-03T06:04:20.794Z INFO zap/logger.go:36 Registered command handler {"command_type": "BulkDeleteNodes"} |
| 2025-09-03 06:04:20.795 | 2025-09-03T06:04:20.794Z INFO zap/logger.go:36 Registered query handler {"query_type": "GetNodeByID"} |
| 2025-09-03 06:04:20.795 | 2025-09-03T06:04:20.794Z INFO zap/logger.go:36 Registered query handler {"query_type": "GetNodesByUser"} |
| 2025-09-03 06:04:20.795 | 2025-09-03T06:04:20.794Z INFO zap/logger.go:36 Registered query handler {"query_type": "SearchNodes"} |
| 2025-09-03 06:04:20.795 | 2025-09-03T06:04:20.795Z INFO zap/logger.go:36 Registered query handler {"query_type": "GetGraph"} |
| 2025-09-03 06:04:20.795 | 2025/09/03 06:04:20 Cold start completed: initialization took 14.290247ms (total cold start: 14.315007ms) |
| 2025-09-03 06:04:20.794 | 2025-09-03T06:04:20.790Z INFO zap/logger.go:36 Registered command handler {"command_type": "CreateNode"} |
| 2025-09-03 06:04:20.794 | 2025-09-03T06:04:20.794Z INFO zap/logger.go:36 Registered command handler {"command_type": "UpdateNode"} |
| 2025-09-03 06:04:20.794 | 2025-09-03T06:04:20.794Z INFO zap/logger.go:36 Registered command handler {"command_type": "DeleteNode"} |
| 2025-09-03 06:04:20.794 | 2025-09-03T06:04:20.794Z INFO zap/logger.go:36 Registered command handler {"command_type": "ConnectNodes"} |
| 2025-09-03 06:04:20.790 | 2025-09-03T06:04:20.790Z INFO di/providers.go:257 EventBridge configuration {"eventBusName": "B2EventBus", "source": "brain2.api", "usingEnvironmentVariable": true} |
| 2025-09-03 06:04:20.790 | 2025-09-03T06:04:20.790Z DEBUG di/providers.go:277 EventBridge publisher configured successfully {"eventBusName": "B2EventBus"} |
| 2025-09-03 06:04:20.789 | 2025/09/03 06:04:20 Processing POST-COLD-START request (22.726051ms after cold start): GET /api/v1/graph-data |
| 2025-09-03 06:04:20.789 | 2025/09/03 06:04:20 DEBUG: GetGraphData called for userID: 125deabf-b32e-4313-b893-4a3ddb416cc2 |
| 2025-09-03 06:04:20.789 | 2025/09/03 06:04:20 DEBUG: Calling queryBus.Send with GetGraphDataQuery |
| 2025-09-03 06:04:20.789 | 2025-09-03T06:04:20.787Z DEBUG zap/logger.go:31 Executing query {"query": "GetGraph"} |
| 2025-09-03 06:04:20.782 | START RequestId: ffbfadde-5803-493b-b5dc-99407195920b Version: $LATEST |
| 2025-09-03 06:04:20.780 | 2025/09/03 06:04:20 Cold start detected - starting Lambda function initialization... |
| 2025-09-03 06:04:20.779 | 2025-09-03T06:04:20.775Z INFO zap/logger.go:36 Registered command handler {"command_type": "CreateNode"} |
| 2025-09-03 06:04:20.779 | 2025-09-03T06:04:20.779Z INFO zap/logger.go:36 Registered command handler {"command_type": "UpdateNode"} |
| 2025-09-03 06:04:20.779 | 2025-09-03T06:04:20.779Z INFO zap/logger.go:36 Registered command handler {"command_type": "DeleteNode"} |
| 2025-09-03 06:04:20.779 | 2025-09-03T06:04:20.779Z INFO zap/logger.go:36 Registered command handler {"command_type": "ConnectNodes"} |
| 2025-09-03 06:04:20.779 | 2025-09-03T06:04:20.779Z INFO zap/logger.go:36 Registered command handler {"command_type": "DisconnectNodes"} |
| 2025-09-03 06:04:20.779 | 2025-09-03T06:04:20.779Z INFO zap/logger.go:36 Registered command handler {"command_type": "BulkDeleteNodes"} |
| 2025-09-03 06:04:20.779 | 2025-09-03T06:04:20.779Z INFO zap/logger.go:36 Registered query handler {"query_type": "GetNodeByID"} |
| 2025-09-03 06:04:20.779 | 2025-09-03T06:04:20.779Z INFO zap/logger.go:36 Registered query handler {"query_type": "GetNodesByUser"} |
| 2025-09-03 06:04:20.779 | 2025-09-03T06:04:20.779Z INFO zap/logger.go:36 Registered query handler {"query_type": "SearchNodes"} |
| 2025-09-03 06:04:20.779 | 2025-09-03T06:04:20.779Z INFO zap/logger.go:36 Registered query handler {"query_type": "GetGraph"} |
| 2025-09-03 06:04:20.779 | 2025/09/03 06:04:20 Cold start completed: initialization took 14.538838ms (total cold start: 14.564981ms) |
| 2025-09-03 06:04:20.775 | 2025-09-03T06:04:20.775Z INFO di/providers.go:257 EventBridge configuration {"eventBusName": "B2EventBus", "source": "brain2.api", "usingEnvironmentVariable": true} |
| 2025-09-03 06:04:20.775 | 2025-09-03T06:04:20.775Z DEBUG di/providers.go:277 EventBridge publisher configured successfully {"eventBusName": "B2EventBus"} |
| 2025-09-03 06:04:20.764 | 2025/09/03 06:04:20 Cold start detected - starting Lambda function initialization... |
| 2025-09-03 06:04:20.743 | 2025/09/03 06:04:20 Processing POST-COLD-START request (25.68667ms after cold start): GET /api/v1/categories |
| 2025-09-03 06:04:20.743 | 2025/09/03 06:04:20 DEBUG: ListCategories called for userID: 125deabf-b32e-4313-b893-4a3ddb416cc2 |
| 2025-09-03 06:04:20.742 | START RequestId: 9d5c0845-9970-49db-8d95-394dd5474479 Version: $LATEST |
| 2025-09-03 06:04:20.734 | 2025-09-03T06:04:20.729Z INFO zap/logger.go:36 Registered command handler {"command_type": "CreateNode"} |
| 2025-09-03 06:04:20.734 | 2025-09-03T06:04:20.734Z INFO zap/logger.go:36 Registered command handler {"command_type": "UpdateNode"} |
| 2025-09-03 06:04:20.734 | 2025-09-03T06:04:20.734Z INFO zap/logger.go:36 Registered command handler {"command_type": "DeleteNode"} |
| 2025-09-03 06:04:20.734 | 2025-09-03T06:04:20.734Z INFO zap/logger.go:36 Registered command handler {"command_type": "ConnectNodes"} |
| 2025-09-03 06:04:20.734 | 2025-09-03T06:04:20.734Z INFO zap/logger.go:36 Registered command handler {"command_type": "DisconnectNodes"} |
| 2025-09-03 06:04:20.734 | 2025-09-03T06:04:20.734Z INFO zap/logger.go:36 Registered command handler {"command_type": "BulkDeleteNodes"} |
| 2025-09-03 06:04:20.734 | 2025-09-03T06:04:20.734Z INFO zap/logger.go:36 Registered query handler {"query_type": "GetNodeByID"} |
| 2025-09-03 06:04:20.734 | 2025-09-03T06:04:20.734Z INFO zap/logger.go:36 Registered query handler {"query_type": "GetNodesByUser"} |
| 2025-09-03 06:04:20.734 | 2025-09-03T06:04:20.734Z INFO zap/logger.go:36 Registered query handler {"query_type": "SearchNodes"} |
| 2025-09-03 06:04:20.734 | 2025-09-03T06:04:20.734Z INFO zap/logger.go:36 Registered query handler {"query_type": "GetGraph"} |
| 2025-09-03 06:04:20.734 | 2025/09/03 06:04:20 Cold start completed: initialization took 16.518041ms (total cold start: 16.547608ms) |
| 2025-09-03 06:04:20.729 | 2025-09-03T06:04:20.729Z INFO di/providers.go:257 EventBridge configuration {"eventBusName": "B2EventBus", "source": "brain2.api", "usingEnvironmentVariable": true} |
| 2025-09-03 06:04:20.729 | 2025-09-03T06:04:20.729Z DEBUG di/providers.go:277 EventBridge publisher configured successfully {"eventBusName": "B2EventBus"} |
| 2025-09-03 06:04:20.718 | 2025/09/03 06:04:20 Cold start detected - starting Lambda function initialization... |
| 2025-09-03 06:04:20.586 | INIT_START Runtime Version: provided:al2.v126 Runtime Version ARN: arn:aws:lambda:us-west-2::runtime:19258f80605e632e71fce9f74eb7eef4421eb4e2c946c551c7eb27c55b4b3c63 |
| 2025-09-03 06:04:20.570 | INIT_START Runtime Version: provided:al2.v126 Runtime Version ARN: arn:aws:lambda:us-west-2::runtime:19258f80605e632e71fce9f74eb7eef4421eb4e2c946c551c7eb27c55b4b3c63 |
| 2025-09-03 06:04:20.521 | END RequestId: 7026835c-477c-4762-805f-d4c985f899cd |
| 2025-09-03 06:04:20.521 | REPORT RequestId: 7026835c-477c-4762-805f-d4c985f899cd Duration: 1473.71 ms Billed Duration: 1792 ms Memory Size: 128 MB Max Memory Used: 96 MB Init Duration: 317.80 ms |
| 2025-09-03 06:04:20.520 | INIT_START Runtime Version: provided:al2.v126 Runtime Version ARN: arn:aws:lambda:us-west-2::runtime:19258f80605e632e71fce9f74eb7eef4421eb4e2c946c551c7eb27c55b4b3c63 |
| 2025-09-03 06:04:20.432 | END RequestId: d97b82b2-f33f-49ba-b775-10ba0a6bbc90 |
| 2025-09-03 06:04:20.432 | REPORT RequestId: d97b82b2-f33f-49ba-b775-10ba0a6bbc90 Duration: 1450.85 ms Billed Duration: 1762 ms Memory Size: 128 MB Max Memory Used: 94 MB Init Duration: 310.68 ms |
| 2025-09-03 06:04:20.430 | END RequestId: 28932d14-6efb-46fe-877f-5584bfd68670 |
| 2025-09-03 06:04:20.430 | REPORT RequestId: 28932d14-6efb-46fe-877f-5584bfd68670 Duration: 1382.87 ms Billed Duration: 1681 ms Memory Size: 128 MB Max Memory Used: 95 MB Init Duration: 297.88 ms |
| 2025-09-03 06:04:20.352 | 2025-09-03T06:04:20.352Z d97b82b2-f33f-49ba-b775-10ba0a6bbc90 INFO Successfully authenticated user: 125deabf-b32e-4313-b893-4a3ddb416cc2 |
| 2025-09-03 06:04:20.341 | 2025-09-03T06:04:20.341Z 7026835c-477c-4762-805f-d4c985f899cd INFO Successfully authenticated user: 125deabf-b32e-4313-b893-4a3ddb416cc2 |
| 2025-09-03 06:04:20.271 | 2025-09-03T06:04:20.271Z 28932d14-6efb-46fe-877f-5584bfd68670 INFO Successfully authenticated user: 125deabf-b32e-4313-b893-4a3ddb416cc2 |
| 2025-09-03 06:04:19.517 | END RequestId: 64054100-8651-4133-a5ec-1e4fed14cd09 |
| 2025-09-03 06:04:19.517 | REPORT RequestId: 64054100-8651-4133-a5ec-1e4fed14cd09 Duration: 871.28 ms Billed Duration: 967 ms Memory Size: 128 MB Max Memory Used: 29 MB Init Duration: 95.68 ms |
| 2025-09-03 06:04:19.515 | 2025/09/03 06:04:19 WebSocket connection established successfully |
| 2025-09-03 06:04:19.062 | 2025-09-03T06:04:19.062Z 7026835c-477c-4762-805f-d4c985f899cd INFO Authorizer invoked with event: {   "version": "2.0",   "type": "REQUEST",   "routeArn": "arn:aws:execute-api:us-west-2:524533608152:f51uq07z1g/$default/GET/api/v1/graph-data",   "identitySource": [     "Bearer eyJhbGciOiJIUzI1NiIsImtpZCI6IndqMzBkQkJGS2thMTg3U0siLCJ0eXAiOiJKV1QifQ.eyJpc3MiOiJodHRwczovL2RoaHV4cGZleWJsbWViZmJwdHphLnN1cGFiYXNlLmNvL2F1dGgvdjEiLCJzdWIiOiIxMjVkZWFiZi1iMzJlLTQzMTMtYjg5My00YTNkZGI0MTZjYzIiLCJhdWQiOiJhdXRoZW50aWNhdGVkIiwiZXhwIjoxNzU2ODgxMDQ1LCJpYXQiOjE3NTY4Nzc0NDUsImVtYWlsIjoiYWRtaW5AdGVzdC5jb20iLCJwaG9uZSI6IiIsImFwcF9tZXRhZGF0YSI6eyJwcm92aWRlciI6ImVtYWlsIiwicHJvdmlkZXJzIjpbImVtYWlsIl19LCJ1c2VyX21ldGFkYXRhIjp7ImVtYWlsX3ZlcmlmaWVkIjp0cnVlfSwicm9sZSI6ImF1dGhlbnRpY2F0ZWQiLCJhYWwiOiJhYWwxIiwiYW1yIjpbeyJtZXRob2QiOiJwYXNzd29yZCIsInRpbWVzdGFtcCI6MTc1NjY2NjYzOH1dLCJzZXNzaW9uX2lkIjoiMThkZWMxNDQtMGI4Mi00NWRmLTgxM2YtYTgzODQ1OGRiNzcyIiwiaXNfYW5vbnltb3VzIjpmYWxzZX0.wp988oU4fEPMi9XTkh4nccuQkKhGD_z82CUuB6Opl7I"   ],   "routeKey": "GET /api/{proxy+}",   "rawPath": "/api/v1/graph-data",   "rawQueryString": "",   "headers": {     "accept": "*/*",     "accept-encoding": "gzip, deflate, br, zstd",     "accept-language": "en-US,en;q=0.9",     "authorization": "Bearer eyJhbGciOiJIUzI1NiIsImtpZCI6IndqMzBkQkJGS2thMTg3U0siLCJ0eXAiOiJKV1QifQ.eyJpc3MiOiJodHRwczovL2RoaHV4cGZleWJsbWViZmJwdHphLnN1cGFiYXNlLmNvL2F1dGgvdjEiLCJzdWIiOiIxMjVkZWFiZi1iMzJlLTQzMTMtYjg5My00YTNkZGI0MTZjYzIiLCJhdWQiOiJhdXRoZW50aWNhdGVkIiwiZXhwIjoxNzU2ODgxMDQ1LCJpYXQiOjE3NTY4Nzc0NDUsImVtYWlsIjoiYWRtaW5AdGVzdC5jb20iLCJwaG9uZSI6IiIsImFwcF9tZXRhZGF0YSI6eyJwcm92aWRlciI6ImVtYWlsIiwicHJvdmlkZXJzIjpbImVtYWlsIl19LCJ1c2VyX21ldGFkYXRhIjp7ImVtYWlsX3ZlcmlmaWVkIjp0cnVlfSwicm9sZSI6ImF1dGhlbnRpY2F0ZWQiLCJhYWwiOiJhYWwxIiwiYW1yIjpbeyJtZXRob2QiOiJwYXNzd29yZCIsInRpbWVzdGFtcCI6MTc1NjY2NjYzOH1dLCJzZXNzaW9uX2lkIjoiMThkZWMxNDQtMGI4Mi00NWRmLTgxM2YtYTgzODQ1OGRiNzcyIiwiaXNfYW5vbnltb3VzIjpmYWxzZX0.wp988oU4fEPMi9XTkh4nccuQkKhGD_z82CUuB6Opl7I",     "content-length": "0",     "content-type": "application/json",     "host": "f51uq07z1g.execute-api.us-west-2.amazonaws.com",     "origin": "https://d28j1yh4i54q1b.cloudfront.net",     "priority": "u=1, i",     "referer": "https://d28j1yh4i54q1b.cloudfront.net/",     "sec-ch-ua": "\"Not;A=Brand\";v=\"99\", \"Microsoft Edge\";v=\"139\", \"Chromium\";v=\"139\"",     "sec-ch-ua-mobile": "?1",     "sec-ch-ua-platform": "\"Android\"",     "sec-fetch-dest": "empty",     "sec-fetch-mode": "cors",     "sec-fetch-site": "cross-site",     "user-agent": "Mozilla/5.0 (Linux; Android 6.0; Nexus 5 Build/MRA58N) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/139.0.0.0 Mobile Safari/537.36 Edg/139.0.0.0",     "x-amzn-trace-id": "Root=1-68b7da62-0f95947127ac131d1d76ca21",     "x-forwarded-for": "73.140.251.80",     "x-forwarded-port": "443",     "x-forwarded-proto": "https"   },   "requestContext": {     "accountId": "524533608152",     "apiId": "f51uq07z1g",     "domainName": "f51uq07z1g.execute-api.us-west-2.amazonaws.com",     "domainPrefix": "f51uq07z1g",     "http": {       "method": "GET",       "path": "/api/v1/graph-data",       "protocol": "HTTP/1.1",       "sourceIp": "73.140.251.80",       "userAgent": "Mozilla/5.0 (Linux; Android 6.0; Nexus 5 Build/MRA58N) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/139.0.0.0 Mobile Safari/537.36 Edg/139.0.0.0"     },     "requestId": "QT8Pch4ivHcEJ2A=",     "routeKey": "GET /api/{proxy+}",     "stage": "$default",     "time": "03/Sep/2025:06:04:18 +0000",     "timeEpoch": 1756879458460   },   "pathParameters": {     "proxy": "v1/graph-data"   } } |
| 2025-09-03 06:04:19.047 | START RequestId: 7026835c-477c-4762-805f-d4c985f899cd Version: $LATEST |
| 2025-09-03 06:04:19.030 | 2025-09-03T06:04:19.030Z 28932d14-6efb-46fe-877f-5584bfd68670 INFO Authorizer invoked with event: {   "version": "2.0",   "type": "REQUEST",   "routeArn": "arn:aws:execute-api:us-west-2:524533608152:f51uq07z1g/$default/GET/api/v1/categories",   "identitySource": [     "Bearer eyJhbGciOiJIUzI1NiIsImtpZCI6IndqMzBkQkJGS2thMTg3U0siLCJ0eXAiOiJKV1QifQ.eyJpc3MiOiJodHRwczovL2RoaHV4cGZleWJsbWViZmJwdHphLnN1cGFiYXNlLmNvL2F1dGgvdjEiLCJzdWIiOiIxMjVkZWFiZi1iMzJlLTQzMTMtYjg5My00YTNkZGI0MTZjYzIiLCJhdWQiOiJhdXRoZW50aWNhdGVkIiwiZXhwIjoxNzU2ODgxMDQ1LCJpYXQiOjE3NTY4Nzc0NDUsImVtYWlsIjoiYWRtaW5AdGVzdC5jb20iLCJwaG9uZSI6IiIsImFwcF9tZXRhZGF0YSI6eyJwcm92aWRlciI6ImVtYWlsIiwicHJvdmlkZXJzIjpbImVtYWlsIl19LCJ1c2VyX21ldGFkYXRhIjp7ImVtYWlsX3ZlcmlmaWVkIjp0cnVlfSwicm9sZSI6ImF1dGhlbnRpY2F0ZWQiLCJhYWwiOiJhYWwxIiwiYW1yIjpbeyJtZXRob2QiOiJwYXNzd29yZCIsInRpbWVzdGFtcCI6MTc1NjY2NjYzOH1dLCJzZXNzaW9uX2lkIjoiMThkZWMxNDQtMGI4Mi00NWRmLTgxM2YtYTgzODQ1OGRiNzcyIiwiaXNfYW5vbnltb3VzIjpmYWxzZX0.wp988oU4fEPMi9XTkh4nccuQkKhGD_z82CUuB6Opl7I"   ],   "routeKey": "GET /api/{proxy+}",   "rawPath": "/api/v1/categories",   "rawQueryString": "",   "headers": {     "accept": "*/*",     "accept-encoding": "gzip, deflate, br, zstd",     "accept-language": "en-US,en;q=0.9",     "authorization": "Bearer eyJhbGciOiJIUzI1NiIsImtpZCI6IndqMzBkQkJGS2thMTg3U0siLCJ0eXAiOiJKV1QifQ.eyJpc3MiOiJodHRwczovL2RoaHV4cGZleWJsbWViZmJwdHphLnN1cGFiYXNlLmNvL2F1dGgvdjEiLCJzdWIiOiIxMjVkZWFiZi1iMzJlLTQzMTMtYjg5My00YTNkZGI0MTZjYzIiLCJhdWQiOiJhdXRoZW50aWNhdGVkIiwiZXhwIjoxNzU2ODgxMDQ1LCJpYXQiOjE3NTY4Nzc0NDUsImVtYWlsIjoiYWRtaW5AdGVzdC5jb20iLCJwaG9uZSI6IiIsImFwcF9tZXRhZGF0YSI6eyJwcm92aWRlciI6ImVtYWlsIiwicHJvdmlkZXJzIjpbImVtYWlsIl19LCJ1c2VyX21ldGFkYXRhIjp7ImVtYWlsX3ZlcmlmaWVkIjp0cnVlfSwicm9sZSI6ImF1dGhlbnRpY2F0ZWQiLCJhYWwiOiJhYWwxIiwiYW1yIjpbeyJtZXRob2QiOiJwYXNzd29yZCIsInRpbWVzdGFtcCI6MTc1NjY2NjYzOH1dLCJzZXNzaW9uX2lkIjoiMThkZWMxNDQtMGI4Mi00NWRmLTgxM2YtYTgzODQ1OGRiNzcyIiwiaXNfYW5vbnltb3VzIjpmYWxzZX0.wp988oU4fEPMi9XTkh4nccuQkKhGD_z82CUuB6Opl7I",     "content-length": "0",     "content-type": "application/json",     "host": "f51uq07z1g.execute-api.us-west-2.amazonaws.com",     "origin": "https://d28j1yh4i54q1b.cloudfront.net",     "priority": "u=1, i",     "referer": "https://d28j1yh4i54q1b.cloudfront.net/",     "sec-ch-ua": "\"Not;A=Brand\";v=\"99\", \"Microsoft Edge\";v=\"139\", \"Chromium\";v=\"139\"",     "sec-ch-ua-mobile": "?1",     "sec-ch-ua-platform": "\"Android\"",     "sec-fetch-dest": "empty",     "sec-fetch-mode": "cors",     "sec-fetch-site": "cross-site",     "user-agent": "Mozilla/5.0 (Linux; Android 6.0; Nexus 5 Build/MRA58N) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/139.0.0.0 Mobile Safari/537.36 Edg/139.0.0.0",     "x-amzn-trace-id": "Root=1-68b7da62-1bf8787a105b0b7f7193392d",     "x-forwarded-for": "73.140.251.80",     "x-forwarded-port": "443",     "x-forwarded-proto": "https"   },   "requestContext": {     "accountId": "524533608152",     "apiId": "f51uq07z1g",     "domainName": "f51uq07z1g.execute-api.us-west-2.amazonaws.com",     "domainPrefix": "f51uq07z1g",     "http": {       "method": "GET",       "path": "/api/v1/categories",       "protocol": "HTTP/1.1",       "sourceIp": "73.140.251.80",       "userAgent": "Mozilla/5.0 (Linux; Android 6.0; Nexus 5 Build/MRA58N) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/139.0.0.0 Mobile Safari/537.36 Edg/139.0.0.0"     },     "requestId": "QT8PcjevvHcEJxA=",     "routeKey": "GET /api/{proxy+}",     "stage": "$default",     "time": "03/Sep/2025:06:04:18 +0000",     "timeEpoch": 1756879458469   },   "pathParameters": {     "proxy": "v1/categories"   } } |
| 2025-09-03 06:04:19.029 | START RequestId: 28932d14-6efb-46fe-877f-5584bfd68670 Version: $LATEST |
| 2025-09-03 06:04:18.983 | 2025-09-03T06:04:18.983Z d97b82b2-f33f-49ba-b775-10ba0a6bbc90 INFO Authorizer invoked with event: {   "version": "2.0",   "type": "REQUEST",   "routeArn": "arn:aws:execute-api:us-west-2:524533608152:f51uq07z1g/$default/GET/api/v1/nodes",   "identitySource": [     "Bearer eyJhbGciOiJIUzI1NiIsImtpZCI6IndqMzBkQkJGS2thMTg3U0siLCJ0eXAiOiJKV1QifQ.eyJpc3MiOiJodHRwczovL2RoaHV4cGZleWJsbWViZmJwdHphLnN1cGFiYXNlLmNvL2F1dGgvdjEiLCJzdWIiOiIxMjVkZWFiZi1iMzJlLTQzMTMtYjg5My00YTNkZGI0MTZjYzIiLCJhdWQiOiJhdXRoZW50aWNhdGVkIiwiZXhwIjoxNzU2ODgxMDQ1LCJpYXQiOjE3NTY4Nzc0NDUsImVtYWlsIjoiYWRtaW5AdGVzdC5jb20iLCJwaG9uZSI6IiIsImFwcF9tZXRhZGF0YSI6eyJwcm92aWRlciI6ImVtYWlsIiwicHJvdmlkZXJzIjpbImVtYWlsIl19LCJ1c2VyX21ldGFkYXRhIjp7ImVtYWlsX3ZlcmlmaWVkIjp0cnVlfSwicm9sZSI6ImF1dGhlbnRpY2F0ZWQiLCJhYWwiOiJhYWwxIiwiYW1yIjpbeyJtZXRob2QiOiJwYXNzd29yZCIsInRpbWVzdGFtcCI6MTc1NjY2NjYzOH1dLCJzZXNzaW9uX2lkIjoiMThkZWMxNDQtMGI4Mi00NWRmLTgxM2YtYTgzODQ1OGRiNzcyIiwiaXNfYW5vbnltb3VzIjpmYWxzZX0.wp988oU4fEPMi9XTkh4nccuQkKhGD_z82CUuB6Opl7I"   ],   "routeKey": "GET /api/{proxy+}",   "rawPath": "/api/v1/nodes",   "rawQueryString": "limit=50",   "headers": {     "accept": "*/*",     "accept-encoding": "gzip, deflate, br, zstd",     "accept-language": "en-US,en;q=0.9",     "authorization": "Bearer eyJhbGciOiJIUzI1NiIsImtpZCI6IndqMzBkQkJGS2thMTg3U0siLCJ0eXAiOiJKV1QifQ.eyJpc3MiOiJodHRwczovL2RoaHV4cGZleWJsbWViZmJwdHphLnN1cGFiYXNlLmNvL2F1dGgvdjEiLCJzdWIiOiIxMjVkZWFiZi1iMzJlLTQzMTMtYjg5My00YTNkZGI0MTZjYzIiLCJhdWQiOiJhdXRoZW50aWNhdGVkIiwiZXhwIjoxNzU2ODgxMDQ1LCJpYXQiOjE3NTY4Nzc0NDUsImVtYWlsIjoiYWRtaW5AdGVzdC5jb20iLCJwaG9uZSI6IiIsImFwcF9tZXRhZGF0YSI6eyJwcm92aWRlciI6ImVtYWlsIiwicHJvdmlkZXJzIjpbImVtYWlsIl19LCJ1c2VyX21ldGFkYXRhIjp7ImVtYWlsX3ZlcmlmaWVkIjp0cnVlfSwicm9sZSI6ImF1dGhlbnRpY2F0ZWQiLCJhYWwiOiJhYWwxIiwiYW1yIjpbeyJtZXRob2QiOiJwYXNzd29yZCIsInRpbWVzdGFtcCI6MTc1NjY2NjYzOH1dLCJzZXNzaW9uX2lkIjoiMThkZWMxNDQtMGI4Mi00NWRmLTgxM2YtYTgzODQ1OGRiNzcyIiwiaXNfYW5vbnltb3VzIjpmYWxzZX0.wp988oU4fEPMi9XTkh4nccuQkKhGD_z82CUuB6Opl7I",     "content-length": "0",     "content-type": "application/json",     "host": "f51uq07z1g.execute-api.us-west-2.amazonaws.com",     "origin": "https://d28j1yh4i54q1b.cloudfront.net",     "priority": "u=1, i",     "referer": "https://d28j1yh4i54q1b.cloudfront.net/",     "sec-ch-ua": "\"Not;A=Brand\";v=\"99\", \"Microsoft Edge\";v=\"139\", \"Chromium\";v=\"139\"",     "sec-ch-ua-mobile": "?1",     "sec-ch-ua-platform": "\"Android\"",     "sec-fetch-dest": "empty",     "sec-fetch-mode": "cors",     "sec-fetch-site": "cross-site",     "user-agent": "Mozilla/5.0 (Linux; Android 6.0; Nexus 5 Build/MRA58N) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/139.0.0.0 Mobile Safari/537.36 Edg/139.0.0.0",     "x-amzn-trace-id": "Root=1-68b7da62-7152b35066da471356bb45e9",     "x-forwarded-for": "73.140.251.80",     "x-forwarded-port": "443",     "x-forwarded-proto": "https"   },   "queryStringParameters": {     "limit": "50"   },   "requestContext": {     "accountId": "524533608152",     "apiId": "f51uq07z1g",     "domainName": "f51uq07z1g.execute-api.us-west-2.amazonaws.com",     "domainPrefix": "f51uq07z1g",     "http": {       "method": "GET",       "path": "/api/v1/nodes",       "protocol": "HTTP/1.1",       "sourceIp": "73.140.251.80",       "userAgent": "Mozilla/5.0 (Linux; Android 6.0; Nexus 5 Build/MRA58N) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/139.0.0.0 Mobile Safari/537.36 Edg/139.0.0.0"     },     "requestId": "QT8PcjetPHcEJxA=",     "routeKey": "GET /api/{proxy+}",     "stage": "$default",     "time": "03/Sep/2025:06:04:18 +0000",     "timeEpoch": 1756879458460   },   "pathParameters": {     "proxy": "v1/nodes"   } } |
| 2025-09-03 06:04:18.981 | START RequestId: d97b82b2-f33f-49ba-b775-10ba0a6bbc90 Version: $LATEST |
| 2025-09-03 06:04:18.857 | END RequestId: 25785075-32bc-423c-bf9b-19b33a80e277 |
| 2025-09-03 06:04:18.857 | REPORT RequestId: 25785075-32bc-423c-bf9b-19b33a80e277 Duration: 601.73 ms Billed Duration: 697 ms Memory Size: 128 MB Max Memory Used: 29 MB Init Duration: 94.33 ms |
| 2025-09-03 06:04:18.823 | 2025/09/03 06:04:18 WebSocket connection cleaned up successfully |
| 2025-09-03 06:04:18.728 | INIT_START Runtime Version: nodejs:20.v75 Runtime Version ARN: arn:aws:lambda:us-west-2::runtime:1ffa4c233e75382c8a39aa96770f3b81af75cba9794a5c2a1750c1ee63cdfe10 |
| 2025-09-03 06:04:18.726 | INIT_START Runtime Version: nodejs:20.v75 Runtime Version ARN: arn:aws:lambda:us-west-2::runtime:1ffa4c233e75382c8a39aa96770f3b81af75cba9794a5c2a1750c1ee63cdfe10 |
| 2025-09-03 06:04:18.667 | INIT_START Runtime Version: nodejs:20.v75 Runtime Version ARN: arn:aws:lambda:us-west-2::runtime:1ffa4c233e75382c8a39aa96770f3b81af75cba9794a5c2a1750c1ee63cdfe10 |
| 2025-09-03 06:04:18.646 | START RequestId: 64054100-8651-4133-a5ec-1e4fed14cd09 Version: $LATEST |
| 2025-09-03 06:04:18.549 | INIT_START Runtime Version: provided:al2.v126 Runtime Version ARN: arn:aws:lambda:us-west-2::runtime:19258f80605e632e71fce9f74eb7eef4421eb4e2c946c551c7eb27c55b4b3c63 |
| 2025-09-03 06:04:18.256 | START RequestId: 25785075-32bc-423c-bf9b-19b33a80e277 Version: $LATEST |
| 2025-09-03 06:04:18.160 | INIT_START Runtime Version: provided:al2.v126 Runtime Version ARN: arn:aws:lambda:us-west-2::runtime:19258f80605e632e71fce9f74eb7eef4421eb4e2c946c551c7eb27c55b4b3c63 |
---