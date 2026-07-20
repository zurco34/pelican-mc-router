# ADR-0002: Inventory is the Source of Truth

## Status

Accepted

## Date

2026-07-20

## Context

The application continuously synchronizes Pelican with one or more routing backends.

The state of the routing system should not depend on the current configuration of Infrared or any other proxy.

## Decision

The application maintains an internal Inventory representing all discovered Minecraft servers.

The Inventory is the authoritative source of truth for:

- Server discovery
- Route generation
- Dashboard
- REST API
- Health checks

Router backends receive generated routes derived from the Inventory.

## Consequences

Infrastructure components become replaceable.

Future routing backends can be added without changing discovery logic.

The dashboard and REST API always operate on a consistent internal model.