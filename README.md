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

## Hosting

The recommended way to run this service is with Docker.

Create a `.env` file in the project root based on `env.tpl` and set the required values before starting the container.

### Environment variables

| Variable             | Description                                 | Example                        |
| -------------------- | ------------------------------------------- | ------------------------------ |
| `IO_BROKER_HOST`     | Address of the ioBroker socket.io instance. | `192.168.0.98:8082/socket.io/` |
| `IO_BROKER_INSTANCE` | Name of the ioBroker datasource             | `system.adapter.influxdb.0`    |
| `DEBUG`              | Shows send and received messages            | `true`                         |
