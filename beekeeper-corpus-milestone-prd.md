# Beekeeper milestone: adjudicated corpus and push containment

Owner: bantuson
License: Apache 2.0
Status: scoped, ready to build
Target: v1 ships at launch (~1 week). v1.1 to 1.9 is the post-launch iteration band for org and team self-host aggregation and push. v2.0 ships the community shared corpus feed.

## 1. Why this milestone

The moat is the adjudicated corpus: incidents with ground truth attached. The behavior and decision layers of an event are cheap to produce and cheap for a competitor to copy. The outcome layer, the confirmed true_label and how it was established, is the expensive and irreplaceable part. Two teams can run byte-identical Sentry detection. The one that adjudicates and keeps the labels owns an asset. The one that does not owns a pile of logs.

Two consequences drive the version split.

The outcome layer is the only part that cannot be retrofitted. You cannot go back and label incidents you already discarded. So the schema must capture the outcome and corroboration fields from the first run, on a single machine, with zero network. That is the meaning of "moat from the start," and it is a v1 requirement.

Distribution makes the moat propagate but is not the moat itself. A confirmed incident is most valuable in the first hour, which is the window that pull-based catalog sync misses. Push-based containment beats pull-based patching on time-to-containment, but it is a remote-influence channel into other machines and carries blast radius. It is built and hardened after launch, not rushed into v1.

## 2. Versioning and shipping strategy

| Version | Scope | Ships | New maintained cloud |
|---|---|---|---|
| v1 | Local corpus loop, moat-grade schema, push envelope format defined | Launch | None |
| v1.1 to 1.9 | Org and team self-host aggregation and push, iterated and hardened | Post-launch band | None (org runs its own aggregator) |
| v2.0 | Community shared corpus feed, maintained push host | Later | Yes (we run the shared host) |

The v1.1 to 1.9 range is an iteration band, not nine public releases. Self-host aggregation and push get built and hardened across these point versions before v2.0 takes on the harder community case.

Enforcement never depends on cloud in any version. The policy engine, Sentry detection, First Responder, and the install-time catalog block all run locally, offline, fail-closed, against the last synced catalog. The corpus loop and push are out of the hot path.

## 3. v1 scope (build now)

### 3.1 Moat-grade event schema

One event record, four layers. Solid fields are always present. Conditional fields populate only for the relevant source_surface.

```
behavior:
  source_surface        # branch key: hook | mcp_gateway | shim | file_watcher | sentry | scan
  action_type
  actor_lineage         # parent-PID ancestry chain
  target_resource
  network_destination
  agent_id              # conditional: agent-mediated surfaces only

decision:
  verdict               # allow | warn | block | alert (alert = Sentry detection-only)
  policy_matched
  rule_id               # conditional: Sentry rule e.g. SENTRY-005
  correlation_window    # conditional: cluster events
  confidence
  ruleset_version

outcome:                # the moat. captured from first run.
  was_correct
  true_label            # malicious | benign | policy_correct | unresolved
  adjudication_source   # see 3.2
  resolved_at

context:
  cluster_id            # binds correlated events into one incident
  baseline_deviation
  repo_fingerprint      # hashed, non-reversible
  fleet_node_id         # anonymized
  scope                 # org_only | community_shareable  (see 3.6)
```

Non-agent incidents (file_watcher, sentry, scan) adjudicate the cluster keyed by cluster_id, not the single event.

### 3.2 Adjudication engine

Assigns the outcome layer after the verdict, asynchronously, off the hot path.

true_label values: malicious, benign, policy_correct (deterministic policy hit where no attack occurred), unresolved (pending).

adjudication_source values:
- catalog_confirmation: package or extension later confirmed by a tracked catalog (Bumblebee, OSV, Socket, Pollen)
- forensic_review: local or analyst inspection of payload or behavior
- breach_confirmation: downstream incident validated the alert
- user_override: developer allowed a block (weak signal, likely false positive)
- downstream_clean: an allow that produced no subsequent incident
- benign_explained: apparent exfil traced to a known-good source

Confidence follows the existing 2FA-for-threat-intelligence rule. One source is warn weight. Two independent sources is enforce weight. source_count is recorded so downstream consumers can apply the warn versus enforce distinction without re-deriving it.

### 3.3 Local corpus store

Append-only local store, extends the existing NDJSON audit log with the outcome layer. Owner-only, same sink model as the audit log (syslog, OTLP, HTTPS). No transmission off-machine in v1. Records are emitted in the push envelope shape (3.5) so later transport is wiring, not redesign.

### 3.4 First Responder integration

First Responder already extends quarantine from extensions to packages with purge and restore in the TUI, firing on alert. v1 wires it to the corpus:
- A confirmed-malicious adjudication arms a quarantine card in the TUI for any matching install present locally.
- Purge stays a local human-confirmed action or an org-policy-gated action. Purge is never an automatic fleet-pushable action in any version.
- Restore remains available for reversibility.
- Red marks attacker actions, coral marks Beekeeper responses, per the locked TUI semantic.

### 3.5 Push envelope format (defined, not transmitted)

Define the wire format now and emit local records in it. Do not stand up any transport in v1.

```
push_envelope:
  signature:
    package_or_extension_id
    version
    behavior_signature_hash
    iocs                  # domains, DNS-tunnel pattern, dead-drop pattern
  true_label
  confidence_tier         # watch (1 source) | enforce (2+ sources)
  source_count
  scope                   # org_only | community_shareable
  action_hint             # watch_and_block ONLY in the pushable set
  signing:                # populated when transport exists
    issuer
    signature
    issued_at
    nonce
```

action_hint constraint: the only fleet-pushable action is watch_and_block (raise Sentry watch, block new pulls of this version, arm the local quarantine card). auto_purge is never present in a pushable envelope. Destructive action stays local.

### 3.6 Scope tagging

Every record carries scope from birth: org_only or community_shareable. This avoids a re-tagging migration when both push paths exist later. Default is org_only. Promotion to community_shareable is explicit and gated by the anonymization step (v2.0), never automatic.

### Non-goals for v1

- No network transmission of corpus data.
- No org aggregator, no push fan-out, no community host.
- No auto-purge across machines, ever.
- SENTRY-008 CI runner OIDC theft and GitHub API dead-drop remain documented residual gaps (section 8), not v1 features.

## 4. v1 phase plan (GSD gates)

Generator builds each phase. Evaluator gate must pass before the next phase starts.

Phase 0: schema and envelope lock
- Generator: finalize 3.1 schema and 3.5 envelope, including scope and corroboration fields.
- Evaluator gate: every field in the Nx Console worked trace maps to a schema field with no gaps. Envelope can represent a watch_and_block push with confidence_tier and source_count. Sign-off freezes the format.

Phase 1: corpus store and adjudication engine
- Generator: append-only store extending the audit log, adjudication engine assigning true_label and adjudication_source, corroboration-based confidence.
- Evaluator gate: a synthetic incident records all four layers. A two-source adjudication is marked enforce weight, a one-source adjudication is marked warn weight. Records emit in envelope shape.

Phase 2: First Responder corpus binding
- Generator: confirmed-malicious adjudication arms the TUI quarantine card, purge gated to human-confirm or org-policy, restore intact.
- Evaluator gate: a confirmed local Nx Console match arms the card and does not auto-purge. Restore reverses a purge cleanly.

Phase 3: launch readiness
- Generator: end-to-end run of the Nx Console incident through trace, record, adjudication, signature, and local feedback to catalog and Sentry watch.
- Evaluator gate: the eight Sentry patterns each produce a moat-grade record. No corpus data leaves the machine. Offline run is fully protective. Documentation states local-first and names residual gaps.

## 5. v1 acceptance criteria

- Schema captures the outcome and corroboration fields on the first run.
- Adjudication engine assigns true_label and adjudication_source and applies warn versus enforce by source_count.
- Local corpus store is append-only, owner-only, never transmitted.
- First Responder arms quarantine cards from confirmed adjudications. Purge stays local and human or policy gated.
- Records are emitted in the frozen push envelope shape with scope tagging.
- Disconnected machine remains fully protective on the last synced catalog.

## 6. v1.1 to 1.9 scope (self-host aggregation and push)

Built and hardened post-launch. Org runs its own aggregator, so no maintained cloud on our side.

- Org aggregator: self-hosted endpoint receiving signed signatures from org machines, building an org-scoped corpus.
- Push fan-out within org: a confirmed incident becomes a signed push. Receiving machines run a Bumblebee or Pollen scan scoped to the specific package or extension version. If found, Sentry raises watch on the process tree, the watcher blocks the agent from pulling that version, and First Responder arms the quarantine card.
- Warn versus enforce gating: one source pushes at watch weight, which only raises detection and arms cards. Two-source corroboration is required before any signal carries enforce weight, and even enforce never triggers automatic destructive action. Purge stays human or org-policy gated.
- Signed channel: authenticated pub/sub fan-out, per-push signature verification, so a machine can prove a push came from the org aggregator and a compromised relay cannot forge actions.
- Offline fallback: a machine without the push channel falls back to existing pull-sync behavior.
- Iterate and harden across the 1.1 to 1.9 band before v2.0.

Time-to-containment claim, stated honestly: this beats pull-based catalogs on containment latency for confirmed incidents within the opted-in fleet, because we have a push channel and an actuator. It does not make Beekeeper a broader or more authoritative catalog than OSV. The win is faster containment on confirmed signals across a known fleet.

## 7. v2.0 scope (community shared corpus feed)

The harder case, with maintained infrastructure on our side.

- Maintained shared host: we run the community ingestion and fan-out endpoint.
- Anonymization step: promotes a record from org_only to community_shareable by stripping to threat intelligence only. The signature describes the attacker (package or extension identity, behavior hash, IOCs) and never the victim (no raw traces, no file or repo contents, no machine identity). fleet_node_id is anonymized.
- Community catalog as a pull source rides the existing catalog sync alongside Bumblebee, OSV, Socket, and Pollen. No new pull cloud beyond the host itself.
- Community push fan-out puts opted-in machines on high alert on a confirmed incident, same watch_and_block constraint, same corroboration gate.
- Abuse resistance: signed pushes, corroboration before enforce weight, rate limiting, resistance to poisoned signatures. The host is a remote-influence channel into stranger machines and is treated as the highest-blast-radius component in the product.

## 8. Risks and residual gaps

- Push channel blast radius: a fleet-wide action channel is itself an attack surface. Mitigated by watch_and_block as the only pushable action, corroboration before enforce weight, signed and verified pushes, and purge kept local.
- SENTRY-008 CI runner OIDC theft: tokens extracted from runner process memory on a machine with no editor and often no host agent. Largely outside host scope. Mitigation is architectural (scoped and ephemeral tokens, trusted-publisher policy), not detection. Name it in the threat model.
- GitHub API dead-drop exfil: a host-level tool cannot reliably distinguish a legitimate push from malicious repo creation with stolen secrets. Architectural mitigation, not detection. Name it.
- Catalog lag: tracked catalogs lag exfil by an hour or more. The push path exists precisely to close this gap for confirmed incidents within the fleet.

## 9. Open decisions

- Confirm SENTRY-001 to 008 numbering against the locked rule spec. This PRD assumes 001 to 005 are the v1.0 rules R1 to R5 and 006 to 008 are the gap-closers (package-manager-descendant, settings.json tripwire, CI runner OIDC).
- Confirm the corpus store reuses the audit log sink configuration or takes its own.
- Confirm default scope is org_only with explicit promotion, as specced in 3.6.
