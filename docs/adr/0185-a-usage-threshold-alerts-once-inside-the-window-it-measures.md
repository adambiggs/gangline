---
id: 0185
status: accepted
date: 2026-09-09
supersedes: []
superseded-by: []
tags: [usage, alerts]
---

# ADR-0185: A usage threshold alerts once inside the window it measures

## Context

A weekly allowance is spent over days, so a reading past a threshold stays
past it, and an alert repeated every pass reads as decoration within the hour.
Session files outlive the window they recorded and replay their last snapshot,
so a reading can measure an allowance already replaced.

## Decision

A threshold raises one alert the first time a reading reaches it inside one
provider window, and none again while that window stands. A window is the reset
time the provider published with it, so thresholds re-arm when the allowance is
replaced, never on a schedule of Gangline's own. A reading whose reset has
passed, or precedes the newest seen, is kept as history, never as the reported
figure. One pass at a time folds a reading in,
and an alert commits with the state that suppresses its repeat.

## Consequences

Thresholds default to 70 and 90 percent and are the operator's to set. An
allowance replaced without its reset time advancing goes unnoticed.

This record is false if a threshold fires twice inside one window whose reset
time is unchanged, or if a reading whose reset has passed becomes the reported
figure.
