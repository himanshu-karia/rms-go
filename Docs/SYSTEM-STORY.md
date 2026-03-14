# RMS-Go System Story (Why this exists)

## Context
KUSUMC firmware is tied to a legacy government MQTT contract and cannot be treated as a moving target.

## Decision
Instead of forcing KUSUMC into generalized platform contracts, we keep a dedicated RMS-Go workspace where:
- legacy topic/payload behavior is first-class
- compatibility overlays are optional and explicit
- deployment/risk decisions are documented for this product line only

## Story arc
1. Base stack existed but had mixed assumptions across legacy + platform models.
2. We aligned firmware docs and backend behavior around legacy-first operation.
3. We hardened operational reliability (token enforcement, dead-letter replay, diagnostics).
4. We standardized Linux-native and compose-aware run/test scripts.
5. We introduced canonical documentation + archive boundaries for handover.

## Outcome
A new contributor can now operate RMS-Go without traversing unrelated workspace docs/code, while still understanding why certain constraints (topic model, replay strategy, URL profile behavior) exist.
