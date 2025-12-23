---
title: Shared Infra
description: ADR on shared database and cache for operational simplicity and data consistency
---

# ADR-07: Shared Infrastructure Strategy

> [한국어 버전](/ko/adr/07-shared-infrastructure.md)

| Date       | Author       | Repos |
| ---------- | ------------ | ----- |
| 2024-12-17 | @KubrickCode | All   |

## Context

### Problem Statement

In distributed systems with multiple services, a fundamental architectural decision is how to manage shared infrastructure components like databases and caches. This decision affects:

1. **Data Consistency**: How to maintain data integrity across services
2. **Operational Complexity**: Number of infrastructure components to manage
3. **Cost Efficiency**: Resource utilization and infrastructure spending
4. **Service Independence**: Ability to deploy and scale services independently
5. **Failure Isolation**: Blast radius when infrastructure components fail

### The Data Infrastructure Spectrum

| Approach               | Data Consistency | Operational Overhead | Service Independence | Cost   |
| ---------------------- | ---------------- | -------------------- | -------------------- | ------ |
| Shared Database        | Strong (ACID)    | Low                  | Low                  | Low    |
| Schema per Service     | Strong (ACID)    | Low-Medium           | Medium               | Low    |
| Database per Service   | Eventual         | High                 | High                 | High   |
| Hybrid (Read Replicas) | Mixed            | Medium               | Medium               | Medium |

### Key Decision Factors

- **Team Structure**: Single team vs. independent service teams
- **Data Relationships**: Frequency of cross-service data operations
- **Consistency Requirements**: Strong consistency needs vs. eventual consistency tolerance
- **Operational Capacity**: Available DevOps/DBA expertise
- **Scale Requirements**: Current and projected traffic patterns
- **Compliance Needs**: Data isolation or multi-tenancy requirements

### Core Question

For systems with multiple services requiring access to shared state, how should database and cache infrastructure be organized to balance operational simplicity with architectural flexibility?

## Decision

**Adopt a shared infrastructure strategy where all services access common database and cache instances.**

Core principles:

- **Operational Simplicity**: Single point of management for data infrastructure
- **Data Consistency**: Leverage ACID transactions across service boundaries
- **Cost Efficiency**: Maximize resource utilization through sharing
- **Pragmatic Trade-offs**: Accept coupling in exchange for reduced complexity
- **Evolution Path**: Design for future decomposition when needed

## Options Considered

### Option A: Shared Infrastructure (Selected)

All services connect to a single database instance and shared cache layer.

**Pros:**

- **Operational Simplicity**: Single database to monitor, backup, and maintain
- **Strong Consistency**: ACID transactions span multiple service data without coordination
- **Cost Efficient**: No duplicate infrastructure, connection pools shared
- **Simpler Queries**: Cross-service data joins without API orchestration
- **Unified Schema**: Single source of truth, easier to understand
- **Reduced Latency**: No network hops for cross-service data access

**Cons:**

- **Tight Coupling**: Schema changes require coordination across all services
- **Shared Fate**: Database failure affects all services simultaneously
- **Scaling Constraints**: Cannot scale database independently per service
- **Resource Contention**: Heavy queries from one service impact others
- **Limited Technology Choice**: All services must use compatible data models
- **Migration Complexity**: Harder to extract services later

### Option B: Database per Service

Each service owns and manages its dedicated database instance.

**Pros:**

- **Service Independence**: Each team controls their data model and schema evolution
- **Failure Isolation**: Database issues contained to individual services
- **Independent Scaling**: Scale database resources per service needs
- **Technology Freedom**: Each service can choose optimal database technology
- **Clear Ownership**: No ambiguity about data responsibility
- **Team Autonomy**: Independent deployment and migration schedules

**Cons:**

- **Operational Overhead**: Multiple databases to manage, monitor, and backup
- **Data Consistency**: Distributed transactions or eventual consistency required
- **Higher Costs**: More instances, connections, and infrastructure
- **Query Complexity**: Cross-service data requires API calls or event synchronization
- **Data Duplication**: Denormalization often required
- **Coordination Overhead**: Inter-service communication patterns needed

### Option C: Schema per Service

Single database instance with isolated schemas for each service.

**Pros:**

- **Logical Separation**: Clear boundaries while sharing infrastructure
- **Migration Path**: Easier transition to full separation later
- **Cost Efficient**: Single instance with logical isolation
- **Some Independence**: Schema changes contained within service schema
- **Balanced Trade-off**: Middle ground between sharing and isolation

**Cons:**

- **Still Shared Fate**: Instance-level failures affect all services
- **Resource Competition**: Shared compute and I/O resources
- **Temptation to Cross Boundaries**: Easy to create cross-schema dependencies
- **Partial Benefits**: Neither full isolation nor full sharing benefits

### Option D: Hybrid with Read Replicas

Shared primary database with service-specific read replicas or caches.

**Pros:**

- **Read Scalability**: Distribute read load across replicas
- **Write Consistency**: Single source of truth for writes
- **Gradual Evolution**: Path toward further decomposition
- **Performance Optimization**: Service-specific read optimization

**Cons:**

- **Replication Lag**: Eventual consistency for reads
- **Complexity**: Multiple data sources to manage
- **Cost**: Additional replica instances
- **Partial Solution**: Doesn't address write scaling

## Consequences

### Positive

1. **Simplified Operations**
   - Single database instance to monitor and maintain
   - Unified backup and disaster recovery procedures
   - Centralized performance tuning and optimization
   - Reduced infrastructure alert fatigue

2. **Data Integrity**
   - ACID transactions ensure consistency across service data
   - No distributed transaction coordination needed
   - Referential integrity enforced at database level
   - Single source of truth eliminates synchronization issues

3. **Cost Efficiency**
   - Lower infrastructure costs through resource sharing
   - Shared connection pools reduce overhead
   - Single managed database service subscription
   - Economies of scale for larger instance sizes

4. **Development Velocity**
   - Cross-service queries without API orchestration
   - Familiar relational patterns for complex data relationships
   - Simplified local development with single database
   - Reduced cognitive load for understanding data flow

5. **Observability**
   - Unified query logs and performance metrics
   - Single point for slow query analysis
   - Holistic view of data access patterns
   - Simplified debugging across service boundaries

### Negative

1. **Service Coupling**
   - Schema changes require cross-team coordination
   - Deployment dependencies between services
   - **Mitigation**: Establish clear schema ownership, use backward-compatible changes, implement schema change review process

2. **Blast Radius**
   - Database failure impacts all dependent services
   - Maintenance windows affect entire system
   - **Mitigation**: High-availability configuration, automated failover, comprehensive backup strategy, circuit breakers in services

3. **Scaling Limitations**
   - Cannot scale database independently per service workload
   - Resource-intensive operations affect all services
   - **Mitigation**: Query optimization, caching strategies, read replicas for read-heavy workloads, connection pooling

4. **Technology Constraints**
   - All services must work with same database technology
   - Cannot optimize storage for different data patterns
   - **Mitigation**: Use database features flexibly (JSON columns, etc.), plan migration path for specialized needs

5. **Long-term Flexibility**
   - Harder to extract services for independent scaling
   - Team autonomy limited by shared data model
   - **Mitigation**: Clean data access layer abstractions, document service boundaries in schema, plan decomposition triggers

### Technical Implications

| Aspect                 | Implication                                                       |
| ---------------------- | ----------------------------------------------------------------- |
| **Schema Management**  | Single migration path, requires coordination for breaking changes |
| **Connection Pooling** | Shared pool across services, requires appropriate sizing          |
| **Query Performance**  | Cross-service impact, requires query optimization discipline      |
| **Backup/Recovery**    | Single backup strategy, unified point-in-time recovery            |
| **Security**           | Shared access controls, service-level permissions at app layer    |
| **Monitoring**         | Unified metrics, easier correlation, shared alerting              |
| **Local Development**  | Simpler setup, single database container/instance                 |
| **Testing**            | Shared test database, requires test isolation strategies          |

### Cache Infrastructure Considerations

Shared caching (Redis/Memcached) follows similar trade-offs:

| Consideration     | Shared Cache                              | Per-Service Cache                |
| ----------------- | ----------------------------------------- | -------------------------------- |
| **Key Namespace** | Requires prefixing to avoid collisions    | Natural isolation                |
| **Memory**        | Efficient utilization, potential eviction | Dedicated resources, predictable |
| **Failure**       | Cache miss affects all services           | Isolated failure                 |
| **Invalidation**  | Cross-service cache coordination easier   | Distributed invalidation needed  |

### When to Reconsider

- **Performance Bottlenecks**: Database becomes limiting factor for specific services
- **Team Scaling**: Independent teams need autonomous data ownership
- **Compliance Requirements**: Data isolation mandated by regulations
- **Technology Needs**: Service requires specialized database (graph, time-series, etc.)
- **Scale Divergence**: Services have dramatically different scaling requirements
- **Failure Isolation**: Blast radius reduction becomes critical requirement
- **Schema Conflicts**: Frequent coordination overhead for schema changes
