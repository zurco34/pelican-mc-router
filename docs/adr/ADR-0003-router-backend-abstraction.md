# ADR-0003: Router Backend Abstraction

## Status

Accepted

## Date

2026-07-20

## Context

The first supported router backend is Infrared.

Future releases may support additional routing technologies such as Velocity or HAProxy.

## Decision

Routing backends are implemented behind a common interface.

The core application generates routing information without knowledge of any specific backend.

Each backend is responsible for rendering configuration and performing reloads.

## Consequences

Adding a new routing backend requires implementing a new backend package without modifying the discovery or routing logic.

This keeps the core application independent of infrastructure-specific implementations.