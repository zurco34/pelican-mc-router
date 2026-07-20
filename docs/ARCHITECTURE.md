# Architecture

## Request Flow

Minecraft Client
        │
        ▼
Traefik
        │
        ▼
Infrared
        │
        ▼
Pelican Server

## Internal Flow

Pelican API
      │
      ▼
Route Manager
      │
      ▼
Infrared Config Generator