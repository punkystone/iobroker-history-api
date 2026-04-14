# iobroker-history-api

HTTP API with a single endpoint for reading history data from charts.

## Endpoint

| Method | URL        | Description      |
| ------ | ---------- | ---------------- |
| `POST` | `/history` | retrieve history |

## Request body

JSON object with the following fields:

- `id` - chart identifier
- `start` - start timestamp
- `end` - end timestamp
- `count` - maximum number of values to request (not accurate)

## Response

On success, the API returns:

- `success` - request status
- `data` - list of history points as timestamp / value key pair

## Errors

- `405 Method Not Allowed` - request method is not `POST`
- `400 Bad Request` - invalid JSON body
- `400 Bad Request` - history lookup failed
