# Architecture Decision Records

ADRs capture decisions that are expensive to reverse. Each ADR stands alone
and follows a consistent format: **Context → Decision → Consequences**.

| #    | Title                                                                       | Status   |
|------|-----------------------------------------------------------------------------|----------|
| 0001 | [CLI framework: cobra + viper](0001-cli-framework.md)                       | Proposed |
| 0002 | [Single binary, package layout](0002-package-layout.md)                     | Proposed |
| 0003 | [Support both in-cluster and out-of-cluster execution](0003-in-cluster-and-out-of-cluster.md) | Proposed |
| 0004 | [Check registry: small interface, no plugins (yet)](0004-check-registry.md) | Proposed |
| 0005 | [etcd health: three collection modes](0005-etcd-health-collection.md)       | Proposed |
| 0006 | [Output formats and exit-code contract](0006-output-formats.md)             | Proposed |
| 0007 | [Declarative tests as local YAML, not CRDs](0007-declarative-tests-not-crds.md) | Proposed |

## Template

```markdown
# ADR-NNNN: <Title>

- **Status**: Proposed | Accepted | Superseded by ADR-XXXX
- **Date**: YYYY-MM-DD

## Context
What problem are we solving? What constraints apply?

## Decision
What did we decide, in one or two crisp sentences?

## Consequences
- Positive
- Negative
- Things this forecloses
```
