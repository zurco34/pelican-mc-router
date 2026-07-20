# ADR-0004: Pelican Discovery

## Status

Accepted

## Date

2026-07-20

## Context

Pelican manages servers using Eggs.

Minecraft Eggs may change over time, and relying on hardcoded Egg IDs is brittle.

## Decision

Minecraft servers should be detected using Egg metadata whenever possible rather than fixed identifiers.

The discovery process converts Pelican-specific data into the project's internal domain model.

The rest of the application should not depend directly on Pelican API structures.

## Consequences

The project becomes more resilient to changes in Pelican.

Supporting new Minecraft Eggs generally requires no code changes if the metadata remains accurate.

The Pelican client becomes the only package responsible for understanding Pelican's API.