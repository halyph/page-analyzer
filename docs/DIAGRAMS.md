# Architecture Diagrams

## System Overview

```mermaid
graph TB
    User[User]

    subgraph "Presentation Layer"
        CLI[CLI Interface]
        Web[Web UI Handler]
        API[REST API Handler]
    end

    subgraph "Core"
        Analyzer[Analyzer]
        Workers[Worker Pool]
        Cache[Cache]
    end

    subgraph "Infrastructure"
        Redis[(Redis)]
        OTEL[OTEL Collector]
    end

    subgraph "Observability"
        Jaeger[Jaeger]
        Prometheus[Prometheus]
        Grafana[Grafana]
    end

    User --> CLI
    User --> Web
    User --> API

    CLI --> Analyzer
    Web --> Analyzer
    API --> Analyzer

    Analyzer --> Workers
    Analyzer --> Cache

    Cache --> Redis
    Analyzer --> OTEL

    OTEL --> Jaeger
    OTEL --> Prometheus
    Grafana --> Prometheus
```

**Note**: Web UI and REST API are **separate interfaces**, not layered. Both call the analyzer directly. Browser JS only polls `/api/jobs/{id}` for async link check results.

## Analysis Flow (REST API)

```mermaid
sequenceDiagram
    User->>API: POST /api/analyze
    API->>Analyzer: Analyze(url)
    Analyzer->>Cache: Check cache

    alt Cache Hit
        Cache-->>Analyzer: Cached result
    else Cache Miss
        Analyzer->>Fetcher: Fetch HTML
        Fetcher->>Target: HTTP GET
        Target-->>Fetcher: HTML

        Analyzer->>Collectors: Parse tokens

        loop Each token
            Collectors->>Collectors: Extract data
        end

        par Async Link Checking
            Analyzer->>Workers: Check links
            Workers->>Targets: HEAD requests
        end

        Analyzer->>Cache: Store result
    end

    Analyzer-->>API: Result + Job ID
    API-->>User: JSON Response

    Note over User,API: For async jobs, browser polls /api/jobs/{id}
```

## Analysis Flow (Web UI)

```mermaid
sequenceDiagram
    User->>Web: POST form
    Web->>Analyzer: Analyze(url)
    Analyzer->>Cache: Check cache

    alt Cache Hit
        Cache-->>Analyzer: Cached result
    else Cache Miss
        Analyzer->>Fetcher: Fetch HTML
        Analyzer->>Collectors: Parse

        par Async Link Checking
            Analyzer->>Workers: Check links
        end

        Analyzer->>Cache: Store result
    end

    Analyzer-->>Web: Result + Job ID
    Web-->>User: Render HTML page

    Note over User,Web: Browser JS polls /api/jobs/{id} for link check status
```

## Worker Pool

```mermaid
graph LR
    Links[Link Queue] --> W1[Worker 1]
    Links --> W2[Worker 2]
    Links --> W3[Worker N]

    W1 --> T1[Target Site 1]
    W2 --> T2[Target Site 2]
    W3 --> T3[Target Site N]

    T1 --> Results[Results]
    T2 --> Results
    T3 --> Results
```

## Collector Pattern

```mermaid
graph TB
    HTML[HTML Stream] --> Parser[Tokenizer]

    Parser --> VC[Version Collector]
    Parser --> TC[Title Collector]
    Parser --> HC[Headings Collector]
    Parser --> LC[Links Collector]
    Parser --> FC[Forms Collector]

    VC --> Result[Analysis Result]
    TC --> Result
    HC --> Result
    LC --> Result
    FC --> Result
```

## Demo Infrastructure

```mermaid
graph TB
    Browser[Browser :8080] --> App[Analyzer App]

    App --> Redis[(Redis :6379)]
    App --> Collector[OTEL Collector :4318]

    Collector --> Jaeger[Jaeger :16686]
    Collector --> Prometheus[Prometheus :9090]

    Grafana[Grafana :3000] --> Prometheus
```

## Caching Strategy

```mermaid
graph TD
    Request[Request] --> Mode{Cache Mode?}

    Mode -->|memory| LRU[LRU Cache]
    Mode -->|redis| Redis[Redis Cache]
    Mode -->|multi| L1{L1 Cache}

    L1 -->|Hit| Return1[Return]
    L1 -->|Miss| L2{L2 Redis}

    L2 -->|Hit| Promote[Promote to L1]
    L2 -->|Miss| Analyze[Analyze]

    Analyze --> Store[Store in Cache]
```
