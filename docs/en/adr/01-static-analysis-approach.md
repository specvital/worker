---
title: Static Analysis
description: ADR on static AST-based analysis for instant test inventory without CI/CD setup
---

# ADR-01: Static Analysis-Based Instant Analysis

> 🇰🇷 [한국어 버전](/ko/adr/01-static-analysis-approach.md)

| Date       | Author       | Repos |
| ---------- | ------------ | ----- |
| 2024-12-17 | @KubrickCode | All   |

## Context

### Problem Statement

Existing test management tools have significant adoption barriers:

1. **High Setup Complexity**: CI/CD pipeline integration requires authentication, environment variables, and infrastructure configuration
2. **Delayed Time-to-Value**: Users must complete setup before seeing any results (hours to days)
3. **Technical Expertise Required**: DevOps knowledge prerequisite excludes non-technical stakeholders (PMs, QA managers)
4. **Test Execution Dependency**: Most tools require actual test runs, which need proper environments and passing tests

### Market Landscape

| Competitor  | Approach             | Entry Barrier | Time-to-First-Value |
| ----------- | -------------------- | ------------- | ------------------- |
| TestRail    | Manual entry         | Medium        | Minutes (manual)    |
| Qase        | AI-assisted          | Medium        | Minutes (manual)    |
| Testomat.io | CI/CD integration    | High          | Hours-Days          |
| PractiTest  | Enterprise workflows | Very High     | Days-Weeks          |

### Core Question

How can we deliver immediate value to users while minimizing adoption friction?

## Decision

**Adopt static analysis-based instant analysis without CI/CD integration requirement.**

Key implementation:

- Users provide only a GitHub repository URL
- System performs AST-based code analysis using Tree-sitter
- Test inventory is generated without executing any tests
- Results are available within seconds of submission

## Options Considered

### Option A: Static Analysis-Based Instant Analysis (Selected)

URL input → Code fetch → AST parsing → Result generation

**Pros:**

- Zero-friction onboarding (Time-to-Value: seconds)
- No authentication required for public repositories
- No test execution environment needed
- Accessible to non-technical users
- Enables PLG (Product-Led Growth) strategy
- Cost-effective (no compute for test execution)

**Cons:**

- Cannot detect dynamically generated test cases
- No pass/fail execution results
- AST parsing accuracy varies by framework complexity

### Option B: CI/CD Pipeline Integration Required

Integrate into CI pipeline → Execute tests → Report results

**Pros:**

- Complete test execution data (pass/fail, timing, coverage)
- Full support for dynamic/parameterized tests
- Natural private repository access
- Industry-standard approach

**Cons:**

- High setup friction (configuration files, secrets, permissions)
- Time-to-Value: hours to days
- Requires DevOps expertise
- Test environment dependencies
- Higher infrastructure costs

### Option C: AI-Based Test Inference

Use LLM to analyze and infer test structure

**Pros:**

- Can handle unconventional patterns
- Natural language descriptions possible

**Cons:**

- Accuracy uncertainty (false positives/negatives)
- High compute costs
- Latency issues
- Hallucination risks
- Static analysis provides sufficient accuracy

## Consequences

### Positive

1. **Zero-Friction Onboarding**
   - Time-to-Value: ~5 seconds vs hours/days
   - Viral potential: easy sharing and demonstration

2. **PLG (Product-Led Growth) Enablement**
   - "Try before you buy" experience
   - Self-service adoption without sales involvement

3. **Broad Accessibility**
   - Non-developers can access test insights
   - No DevOps knowledge required

4. **Cost Efficiency**
   - No test execution infrastructure

5. **Competitive Differentiation**
   - Unique position: instant static analysis
   - Not competing on CI/CD features

### Negative

1. **Dynamic Test Limitations**
   - Parameterized tests may show incomplete counts
   - Data-driven tests not fully captured
   - **Mitigation**: Clear documentation of limitations, "estimated" labels

2. **No Execution Results**
   - Cannot show pass/fail status
   - No timing or coverage data
   - **Mitigation**: Position as "test inventory" not "test results"

3. **Framework Coverage**
   - Must implement parser for each framework
   - Edge cases in complex test structures
   - **Mitigation**: Prioritize popular frameworks, community contributions

### Technical Implications

| Aspect            | Implication                                                    |
| ----------------- | -------------------------------------------------------------- |
| **Architecture**  | Core library must be framework-agnostic with pluggable parsers |
| **Performance**   | Shallow clone + parallel parsing for large repos               |
| **Scalability**   | Async queue processing for analysis workload                   |
| **Extensibility** | Plugin architecture for new framework support                  |

## References

- [Product Overview](../prd/00-overview.md) - Core value proposition
- [Core Engine](../prd/02-core-engine.md) - Parser implementation details
- [Architecture](../architecture.md) - System design
