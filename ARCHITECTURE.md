# Architecture

## Ownership boundary

A MeshGrid controller represents one administrative domain: a person, family, lab, company, or community. It is authoritative only for agents enrolled into that domain. No global API, root controller, cryptocurrency, marketplace, hosted login, or subscription is required.

## Local reconciliation

1. An agent sends its identity, labels, declared capacity, and observed assignments.
2. The controller persists the observation and reconciles desired replicas.
3. The scheduler filters live nodes by label and capacity, then selects the node with the most free memory and a stable ID tie-breaker.
4. The heartbeat response contains that node's desired assignments.
5. The agent converges Docker to the desired state and reports success or failure.
6. Deletions remain tombstoned until the assigned agent acknowledges container removal.

The API is declarative and idempotent at the assignment layer. A controller restart reloads state from disk; an agent restart reconstructs actual state from Docker on its next reconciliation.

## Federation protocol

Federation is a relationship between equal controllers, never membership in one global cluster. The protocol is designed around four signed document types:

| Document | Producer | Consumer | Required contents |
|---|---|---|---|
| Capacity advertisement | Hosting controller | Approved peers | Coarse free resources, policy class, expiry |
| Lease request | Workload owner | Hosting controller | Workload digest, resources, duration, nonce |
| Lease receipt | Hosting controller | Workload owner | Decision, lease ID, expiry, policy digest |
| Status receipt | Hosting controller | Workload owner | Lease ID, observed state, monotonic sequence |

Every document is signed by an Ed25519 controller identity. Receivers must verify the configured peer key, a short timestamp window, an unconsumed nonce, expiry, quotas, and the current admission policy. The local controller translates an accepted lease into an ordinary local workload marked with its remote owner. Only nodes that opt into federation may receive it.

### No global consensus

Global consensus would recreate the central coordination and partition behavior this project is avoiding. Each owner has strongly consistent local desired state. Cross-owner state uses leases with explicit expiry and signed receipts. A network partition prevents renewal and causes the hosting controller to stop the remote workload after its grace period; it does not block either owner's local cluster.

### Scheduling privacy

Advertisements should expose resource buckets such as `cpu=8-16`, `ram=32-64GiB`, and capability labels rather than exact inventories. A peer learns nothing about local-only nodes or workloads. Operators can create separate federation pools with image allowlists, quotas, time windows, and egress policy.

## Threat model

The current MVP protects its local HTTP API with a high-entropy bearer token and provides controller signing primitives. It assumes the HTTP transport is inside TLS, WireGuard, Tailscale, or another authenticated encrypted network.

Production federation must additionally defend against:

- replayed requests with a durable nonce journal;
- a compromised peer attempting resource exhaustion;
- malicious images, decompression bombs, and mutable image tags;
- secret disclosure to a hosting peer;
- network and filesystem escape from a container;
- forged or reordered status receipts;
- abandoned leases after partitions;
- key theft and peer-key rotation.

Remote execution should remain disabled until these controls are implemented and tested. The code intentionally provides no switch that falsely labels incomplete federation as safe.

## Storage evolution

Atomic JSON is appropriate for the first controller and keeps installation trivial. The storage API is transactional, so it can be replaced with embedded SQLite or Pebble when audit events, nonce journals, and larger clusters require indexed queries. An optional three-controller high-availability mode can replicate only within one ownership domain.

