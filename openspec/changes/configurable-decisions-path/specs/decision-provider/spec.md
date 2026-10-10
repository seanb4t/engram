# Spec Delta

## Purpose

Defines how engram reaches its typed-decision provider: the endpoint that each decision request goes to, and the settings that build it.

## ADDED Requirements

### Requirement: Decision endpoint is the base URL plus the decisions path

engram SHALL send every decision request to `ENGRAM_DECISIONS_BASE_URL` with its trailing slashes removed, followed by `ENGRAM_DECISIONS_PATH`. This SHALL apply to every caller of the provider: `spine-review consolidate` verdicts, search re-ranking, and console query understanding.

#### Scenario: Native LiteLLM route

- **WHEN** `ENGRAM_DECISIONS_BASE_URL` is `https://llm.example.com/` and `ENGRAM_DECISIONS_PATH` is `/v1/systemone`
- **THEN** each decision request is a POST to `https://llm.example.com/v1/systemone`

#### Scenario: Every caller uses the same endpoint

- **WHEN** `ENGRAM_DECISIONS_PATH` is `/v1/systemone` and a consolidate verdict, a re-ranked search and a query-understanding call each make a decision request
- **THEN** all three requests go to `{base}/v1/systemone`

### Requirement: Default decisions path

`ENGRAM_DECISIONS_PATH` SHALL default to `/alpha/decisions`. An unset or empty value SHALL send requests to the same URL that engram used before the setting existed.

#### Scenario: Unset

- **WHEN** `ENGRAM_DECISIONS_PATH` is unset and `ENGRAM_DECISIONS_BASE_URL` is `https://openrouter.ai/api`
- **THEN** each decision request is a POST to `https://openrouter.ai/api/alpha/decisions`

#### Scenario: Set to empty

- **WHEN** `ENGRAM_DECISIONS_PATH` is set to the empty string
- **THEN** the effective path is `/alpha/decisions`

### Requirement: Decisions path validation

When `ENGRAM_DECISIONS_PROVIDER` is `jev`, configuration validation SHALL reject an `ENGRAM_DECISIONS_PATH` that does not start with `/` or that contains `?` or `#`, with an error that names `ENGRAM_DECISIONS_PATH`. When the provider is off, the path SHALL NOT be validated.

#### Scenario: Missing leading slash

- **WHEN** the provider is `jev` and `ENGRAM_DECISIONS_PATH` is `v1/systemone`
- **THEN** startup fails configuration validation with an error naming `ENGRAM_DECISIONS_PATH`, before any decision request is sent

#### Scenario: Full URL instead of a path

- **WHEN** the provider is `jev` and `ENGRAM_DECISIONS_PATH` is `https://llm.example.com/v1/systemone`
- **THEN** startup fails configuration validation with an error naming `ENGRAM_DECISIONS_PATH`

#### Scenario: Query string or fragment

- **WHEN** the provider is `jev` and `ENGRAM_DECISIONS_PATH` is `/v1/systemone?x=1` or `/v1/systemone#a`
- **THEN** startup fails configuration validation with an error naming `ENGRAM_DECISIONS_PATH`

#### Scenario: Provider off

- **WHEN** `ENGRAM_DECISIONS_PROVIDER` is empty and `ENGRAM_DECISIONS_PATH` is `not-a-path`
- **THEN** configuration validation does not report `ENGRAM_DECISIONS_PATH`

### Requirement: The path does not change the request or its handling

The decisions path SHALL change only the request URL. The request body, the headers, the response decoding, the retry policy, the timeouts and the reported failure classes SHALL be the same for every path.

#### Scenario: Same request on a different path

- **WHEN** the same decision request is sent once with the default path and once with `/v1/systemone`
- **THEN** both requests carry the same method, headers and body, and the same response body decodes to the same answers
