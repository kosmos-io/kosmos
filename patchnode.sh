#!/bin/bash

while true
do
  curl -X PATCH http://localhost:8001/api/v1/nodes/host-control-plane/status \
  -H "Content-Type: application/merge-patch+json" \
  -d '{
    "status": {
      "conditions": [
        {
          "type": "Ready",
          "status": "False",
          "reason": "MyReason",
          "message": "My custom message"
        }
      ]
    }
  }'
  sleep 1  
done
