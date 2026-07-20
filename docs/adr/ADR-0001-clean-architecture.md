# ADR-0001: Clean Architecture

## Status

Accepted

## Date

2026-07-20

## Context

Pelican MC Router is intended to be a production-quality, open-source application.

The project will integrate with the Pelican API, generate routing configurations, expose a REST API, provide a web dashboard, and support multiple routing backends in the future.

To remain maintainable, the application must be modular, testable, and avoid tight coupling between components.

## Decision

The project will follow the principles of Clean Architecture.

Key principles include:

- Separation of concerns.
- Dependency inversion.
- Small, focused packages.
- Interface-driven design.
- Dependency injection.
- Clear boundaries between domain logic and infrastructure.
- Infrastructure components (Pelican, Infrared, REST API, Dashboard) must depend on the core domain, never the other way around.

## Consequences

Benefits include:

- Easier testing.
- Easier maintenance.
- Clear package responsibilities.
- Future support for additional routing backends.
- Reduced coupling.
- Improved contributor experience.

The downside is slightly increased initial complexity, which is acceptable for a long-lived open-source project.