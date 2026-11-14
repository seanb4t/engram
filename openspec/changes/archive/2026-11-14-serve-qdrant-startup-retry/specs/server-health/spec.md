# Spec Delta

## Purpose

Defines the liveness and readiness routes of `engram serve`, what each one reports during and after startup, and how the Helm chart probes them.

## ADDED Requirements

### Requirement: Liveness route

`engram serve` SHALL answer `GET /livez` on its primary listener with HTTP 200 from bind until shutdown. The answer SHALL NOT depend on provisioning or on Qdrant. The route SHALL need no authentication.

#### Scenario: Live during provisioning

- **WHEN** `engram serve` is still provisioning its collection and a client sends `GET /livez` with no credentials
- **THEN** the response is HTTP 200

#### Scenario: Live while Qdrant is down after start

- **WHEN** provisioning completed and Qdrant then stops answering
- **THEN** `GET /livez` still returns HTTP 200

### Requirement: Readiness route

`engram serve` SHALL answer `GET /readyz` on its primary listener. If provisioning completed and the most recent background Qdrant health check succeeded, the answer SHALL be HTTP 200. In every other state, the answer SHALL be HTTP 503. The route SHALL need no authentication.

#### Scenario: Not ready during provisioning

- **WHEN** `engram serve` is still provisioning its collection
- **THEN** `GET /readyz` returns HTTP 503

#### Scenario: Not ready before the first background check

- **WHEN** provisioning completed and no background health check finished yet
- **THEN** `GET /readyz` returns HTTP 503

#### Scenario: Ready after provisioning

- **WHEN** provisioning completed and the most recent background health check succeeded
- **THEN** `GET /readyz` returns HTTP 200

#### Scenario: Not ready after a failed health check

- **WHEN** provisioning completed and the most recent background health check failed
- **THEN** `GET /readyz` returns HTTP 503

#### Scenario: Ready again after Qdrant recovers

- **WHEN** a background health check succeeds after one that failed
- **THEN** `GET /readyz` returns HTTP 200

### Requirement: Readiness never calls Qdrant per request

A `GET /readyz` request SHALL NOT send a request to Qdrant. Readiness SHALL report the result of the most recent background health check. `engram serve` runs the first check as soon as provisioning completes, then repeats it on a fixed interval with its own timeout.

#### Scenario: Many probes, no Qdrant traffic

- **WHEN** clients send many `GET /readyz` requests between two background health checks
- **THEN** Qdrant receives no request caused by those probes

### Requirement: Chart probes use the health routes

The Helm chart SHALL probe the server container on its `http` port. The startup and liveness probes SHALL be HTTP GET on `/livez`. The readiness probe SHALL be HTTP GET on `/readyz`.

#### Scenario: Default render

- **WHEN** the chart is rendered with default values
- **THEN** the server container's `startupProbe` and `livenessProbe` are `httpGet` on `/livez` and port `http`
- **AND** its `readinessProbe` is `httpGet` on `/readyz` and port `http`
