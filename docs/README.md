# SMS Gateway Service

A high-performance, scalable SMS gateway service built with Go that provides reliable message delivery through a priority-based queue system with Kafka integration.


## Overview

This SMS Gateway Service is designed to handle high-volume SMS operations with guaranteed delivery, priority-based routing, and comprehensive status tracking. It uses a distributed architecture with Kafka for message queuing, PostgreSQL for transactional data, ClickHouse for analytics, and Redis for caching.

### Key Capabilities

- **Priority-based Routing**: Messages are prioritized based on customer plans (Free, Pro, Enterprise)
- **Fair Queue Management**: Round-robin queue management ensures fair distribution across customers
- **Reliable Delivery**: Dead Letter Queue (DLQ) for failed messages with retry mechanisms
- **Real-time Status Tracking**: Track SMS delivery status in real-time
- **Scalable Architecture**: Horizontal scaling with worker pools and Kafka consumers
- **Balance Management**: Automatic balance deduction and tracking

## Features

- **Multi-tier Priority System**: Free, Pro, and Enterprise plans with different priorities
- **API Key Authentication**: Secure API access with plan-based authentication
- **Worker Pool**: Configurable worker pool for concurrent SMS processing
- **Customer Queue Isolation**: Per-customer queues prevent one customer from blocking others
- **Fault Tolerance**: Dead Letter Queue for handling Kafka write failures
- **Status Timeline**: View complete SMS delivery timeline
- **Pagination Support**: Efficient pagination for SMS logs
- **Database Partitioning**: PostgreSQL table partitioning for better performance
- **TTL Management**: Automatic data cleanup in ClickHouse (90-day retention)
- **Docker Support**: Full Docker Compose setup for easy deployment

## Architecture

### System Architecture

```
┌─────────────┐
│   Client    │
└──────┬──────┘
       │ HTTP POST /v1/sms/send
       │ (X-Api-Key, X-Auth-User-Id)
       ▼
┌─────────────────────────────────┐
│      API Server (Gin)            │
│  ┌───────────────────────────┐  │
│  │  Auth Middleware          │  │
│  │  Priority Middleware      │  │
│  └───────────────────────────┘  │
│  ┌───────────────────────────┐  │
│  │  SMS Handler              │  │
│  │  - Validate Request       │  │
│  │  - Deduct Balance         │  │
│  │  - Enqueue to Workers     │  │
│  └───────────────────────────┘  │
└────────┬────────────────────────┘
         │
         ▼
┌────────────────────────────────────┐
│   Kafka Background Workers          │
│   (Channel-based Work Queue)        │
│   - Retry logic (3 attempts)        │
│   - DLQ fallback on failure         │
└────────┬───────────────────────────┘
         │
         ▼
┌─────────────────────────────────────┐
│   Kafka Topic: sms_accepted         │
└────────┬────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────┐
│   SMS Consumer (Multiple Instances) │
│   - Reads from Kafka                │
│   - Creates Jobs                    │
│   - Enqueues to QueueManager        │
└────────┬────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────┐
│   Queue Manager                      │
│   ┌───────────────────────────────┐ │
│   │  Per-Customer Priority Queues │ │
│   │  - Round-robin selection      │ │
│   │  - Customer locking mechanism │ │
│   └───────────────────────────────┘ │
└────────┬────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────┐
│   Worker Pool                        │
│   - Configurable worker count        │
│   - Processes jobs from queues       │
│   - Sends via SMS Provider           │
│   - Publishes status to Kafka        │
└────────┬────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────┐
│   Kafka Topic: sms_status           │
└────────┬────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────┐
│   Status Consumer                    │
│   - Reads status updates             │
│   - Stores in ClickHouse             │
└─────────────────────────────────────┘
```

### Data Flow

1. **API Request**: Client sends SMS request with API key and user ID
2. **Authentication**: Middleware validates API key and retrieves priority level
3. **Balance Check**: System checks and deducts customer balance
4. **Message Queuing**: SMS is enqueued to background workers via channel
5. **Kafka Publishing**: Workers publish to `sms_accepted` topic (with retry/DLQ)
6. **Consumer Processing**: Consumers read from Kafka and enqueue to Queue Manager
7. **Worker Processing**: Workers dequeue jobs in round-robin fashion and send SMS
8. **Status Tracking**: Status updates published to `sms_status` topic
9. **Analytics Storage**: Status consumer stores logs in ClickHouse

### Components

#### 1. API Server (`server` command)
- REST API endpoints for SMS operations
- Authentication and priority middleware
- Balance management
- Background Kafka workers

#### 2. SMS Consumer (`consume` command)
- Consumes from `sms_accepted` Kafka topic
- Enqueues jobs to Queue Manager
- Publishes processing status

#### 3. Status Consumer (`consume-status` command)
- Consumes from `sms_status` Kafka topic
- Stores status logs in ClickHouse for analytics

#### 4. Queue Manager
- Per-customer queue isolation
- Round-robin customer selection
- Customer locking during processing
- Priority-based ordering

#### 5. Worker Pool
- Configurable number of workers
- Concurrent job processing
- SMS provider integration
- Status publishing


##  Installation

### Clone the Repository

```bash
git clone <repository-url>
cd messenger
```


##  Running the Project

### Using Make Commands

```bash
# View all available commands
make env

# Start all infra services
make infra
#This will start:
#- PostgreSQL (port 5432)
#- Redis (port 6379)
#- Kafka (port 9092)
#- Zookeeper (port 2181)
#- ClickHouse (HTTP: 8123, Native: 9000)
#- Kafka Magic (Web UI: 8997)
#- API Server (port 8080)
#- SMS Consumer
#- Status Consumer

# start migrator service
make migrator-up

# run migrations
make migrate

# start tester container
make tester-up

# charge users with random balances
make charge

# start (server + consumer + status_consumer)
# also http server is running on 8080
make up

# run stress test
make stress

# View logs
make log RUN_ARG=server # ( server OR consumer OR status_consumer)

# Check status
make ps

# Stop all services
make down

# Stop and remove volumes
make purge

# Generate Swagger documentation
make generate-swagger
```


## Swagger Documentation

Generate and view Swagger documentation:

```bash
# install swago
make init
# generate
make generate-swagger
```

Then open `docs/swagger.yaml` or integrate with Swagger UI.

# result of stress test : 
````

Target Server:     http://server:8080
Total Users:       100000
Target RPS:        2000 requests/second
Duration:          5m0s
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

User Distribution:
  free: 50000 users (50.0%)
  pro: 30000 users (30.0%)
  enterprise: 20000 users (20.0%)

🔥 Load test started at 15:08:43

📊 Total:   2479 | Success:   2479 | No-Balance:      0 | Failed:      0 | RPS:  1239.0 | Success Rate: 100.0% | Latency (ms): avg=0 min=0 max=21
📊 Total:   4966 | Success:   4964 | No-Balance:      0 | Failed:      0 | RPS:  1243.4 | Success Rate: 100.0% | Latency (ms): avg=0 min=0 max=21
📊 Total:   7437 | Success:   7436 | No-Balance:      0 | Failed:      0 | RPS:  1236.0 | Success Rate: 100.0% | Latency (ms): avg=0 min=0 max=21
📊 Total:   9809 | Success:   9809 | No-Balance:      0 | Failed:      0 | RPS:  1185.9 | Success Rate: 100.0% | Latency (ms): avg=0 min=0 max=22
📊 Total:  12314 | Success:  12313 | No-Balance:      0 | Failed:      0 | RPS:  1252.4 | Success Rate: 100.0% | Latency (ms): avg=0 min=0 max=31
📊 Total:  14838 | Success:  14835 | No-Balance:      0 | Failed:      0 | RPS:  1261.6 | Success Rate: 100.0% | Latency (ms): avg=0 min=0 max=31
📊 Total:  17341 | Success:  17339 | No-Balance:      0 | Failed:      0 | RPS:  1251.9 | Success Rate: 100.0% | Latency (ms): avg=0 min=0 max=35
📊 Total:  19854 | Success:  19852 | No-Balance:      0 | Failed:      0 | RPS:  1256.6 | Success Rate: 100.0% | Latency (ms): avg=0 min=0 max=38
📊 Total:  22418 | Success:  22405 | No-Balance:      0 | Failed:      0 | RPS:  1281.3 | Success Rate:  99.9% | Latency (ms): avg=0 min=0 max=38
📊 Total:  24866 | Success:  24864 | No-Balance:      0 | Failed:      0 | RPS:  1224.6 | Success Rate: 100.0% | Latency (ms): avg=0 min=0 max=127
📊 Total:  27454 | Success:  27454 | No-Balance:      0 | Failed:      0 | RPS:  1293.6 | Success Rate: 100.0% | Latency (ms): avg=0 min=0 max=127
📊 Total:  30035 | Success:  30033 | No-Balance:      0 | Failed:      0 | RPS:  1290.8 | Success Rate: 100.0% | Latency (ms): avg=0 min=0 max=127
📊 Total:  32632 | Success:  32631 | No-Balance:      0 | Failed:      0 | RPS:  1297.4 | Success Rate: 100.0% | Latency (ms): avg=0 min=0 max=127
📊 Total:  35203 | Success:  35203 | No-Balance:      0 | Failed:      0 | RPS:  1286.6 | Success Rate: 100.0% | Latency (ms): avg=0 min=0 max=127
📊 Total:  37769 | Success:  37769 | No-Balance:      0 | Failed:      0 | RPS:  1283.1 | Success Rate: 100.0% | Latency (ms): avg=0 min=0 max=127
📊 Total:  40374 | Success:  40374 | No-Balance:      0 | Failed:      0 | RPS:  1302.2 | Success Rate: 100.0% | Latency (ms): avg=0 min=0 max=127
📊 Total:  42935 | Success:  42935 | No-Balance:      0 | Failed:      0 | RPS:  1280.5 | Success Rate: 100.0% | Latency (ms): avg=0 min=0 max=127
📊 Total:  45531 | Success:  45530 | No-Balance:      0 | Failed:      0 | RPS:  1298.4 | Success Rate: 100.0% | Latency (ms): avg=0 min=0 max=127
📊 Total:  48054 | Success:  48045 | No-Balance:      0 | Failed:      0 | RPS:  1261.4 | Success Rate: 100.0% | Latency (ms): avg=0 min=1 max=127
📊 Total:  50529 | Success:  50525 | No-Balance:      0 | Failed:      0 | RPS:  1237.7 | Success Rate: 100.0% | Latency (ms): avg=0 min=0 max=127
📊 Total:  53053 | Success:  53052 | No-Balance:      0 | Failed:      0 | RPS:  1261.5 | Success Rate: 100.0% | Latency (ms): avg=0 min=1 max=127
📊 Total:  55531 | Success:  55531 | No-Balance:      0 | Failed:      0 | RPS:  1239.2 | Success Rate: 100.0% | Latency (ms): avg=0 min=1 max=127
📊 Total:  58082 | Success:  58080 | No-Balance:      0 | Failed:      0 | RPS:  1275.2 | Success Rate: 100.0% | Latency (ms): avg=0 min=1 max=127
📊 Total:  60553 | Success:  60552 | No-Balance:      0 | Failed:      0 | RPS:  1236.1 | Success Rate: 100.0% | Latency (ms): avg=0 min=0 max=127
📊 Total:  63009 | Success:  63008 | No-Balance:      0 | Failed:      0 | RPS:  1227.9 | Success Rate: 100.0% | Latency (ms): avg=0 min=0 max=127
📊 Total:  65468 | Success:  65467 | No-Balance:      0 | Failed:      0 | RPS:  1229.6 | Success Rate: 100.0% | Latency (ms): avg=0 min=0 max=127
📊 Total:  68011 | Success:  68010 | No-Balance:      0 | Failed:      0 | RPS:  1271.2 | Success Rate: 100.0% | Latency (ms): avg=0 min=1 max=127
📊 Total:  70614 | Success:  70613 | No-Balance:      0 | Failed:      0 | RPS:  1301.6 | Success Rate: 100.0% | Latency (ms): avg=0 min=0 max=127
📊 Total:  73176 | Success:  73173 | No-Balance:      0 | Failed:      0 | RPS:  1281.3 | Success Rate: 100.0% | Latency (ms): avg=0 min=0 max=127
📊 Total:  75701 | Success:  75700 | No-Balance:      0 | Failed:      0 | RPS:  1261.8 | Success Rate: 100.0% | Latency (ms): avg=0 min=0 max=127
📊 Total:  78067 | Success:  78063 | No-Balance:      0 | Failed:      0 | RPS:  1183.5 | Success Rate: 100.0% | Latency (ms): avg=0 min=0 max=127
📊 Total:  80592 | Success:  80589 | No-Balance:      0 | Failed:      0 | RPS:  1262.6 | Success Rate: 100.0% | Latency (ms): avg=1 min=1 max=127
📊 Total:  83202 | Success:  83192 | No-Balance:      0 | Failed:      0 | RPS:  1304.5 | Success Rate: 100.0% | Latency (ms): avg=1 min=1 max=127
📊 Total:  85702 | Success:  85700 | No-Balance:      0 | Failed:      0 | RPS:  1249.9 | Success Rate: 100.0% | Latency (ms): avg=1 min=1 max=127
📊 Total:  88237 | Success:  88234 | No-Balance:      0 | Failed:      0 | RPS:  1268.0 | Success Rate: 100.0% | Latency (ms): avg=1 min=1 max=127
📊 Total:  90778 | Success:  90777 | No-Balance:      0 | Failed:      0 | RPS:  1270.5 | Success Rate: 100.0% | Latency (ms): avg=1 min=0 max=127
📊 Total:  93361 | Success:  93357 | No-Balance:      0 | Failed:      0 | RPS:  1291.4 | Success Rate: 100.0% | Latency (ms): avg=1 min=1 max=127
📊 Total:  95914 | Success:  95914 | No-Balance:      0 | Failed:      0 | RPS:  1276.5 | Success Rate: 100.0% | Latency (ms): avg=1 min=1 max=127
📊 Total:  98484 | Success:  98479 | No-Balance:      0 | Failed:      0 | RPS:  1285.0 | Success Rate: 100.0% | Latency (ms): avg=1 min=1 max=127
📊 Total: 100941 | Success: 100939 | No-Balance:      1 | Failed:      0 | RPS:  1228.5 | Success Rate: 100.0% | Latency (ms): avg=1 min=0 max=127
📊 Total: 103396 | Success: 103389 | No-Balance:      6 | Failed:      0 | RPS:  1227.1 | Success Rate: 100.0% | Latency (ms): avg=1 min=1 max=127
📊 Total: 105957 | Success: 105946 | No-Balance:     10 | Failed:      0 | RPS:  1281.0 | Success Rate: 100.0% | Latency (ms): avg=1 min=1 max=127
📊 Total: 108570 | Success: 108556 | No-Balance:     13 | Failed:      0 | RPS:  1306.5 | Success Rate: 100.0% | Latency (ms): avg=1 min=0 max=127
📊 Total: 111124 | Success: 111108 | No-Balance:     16 | Failed:      0 | RPS:  1277.0 | Success Rate: 100.0% | Latency (ms): avg=1 min=0 max=127
📊 Total: 113715 | Success: 113694 | No-Balance:     19 | Failed:      0 | RPS:  1295.4 | Success Rate: 100.0% | Latency (ms): avg=1 min=1 max=127
📊 Total: 116277 | Success: 116254 | No-Balance:     21 | Failed:      0 | RPS:  1280.9 | Success Rate: 100.0% | Latency (ms): avg=1 min=0 max=127
📊 Total: 118543 | Success: 118505 | No-Balance:     25 | Failed:      0 | RPS:  1131.4 | Success Rate: 100.0% | Latency (ms): avg=1 min=2 max=127
📊 Total: 121082 | Success: 121052 | No-Balance:     28 | Failed:      0 | RPS:  1271.4 | Success Rate: 100.0% | Latency (ms): avg=1 min=0 max=127
📊 Total: 123608 | Success: 123567 | No-Balance:     34 | Failed:      0 | RPS:  1262.5 | Success Rate: 100.0% | Latency (ms): avg=1 min=3 max=127
📊 Total: 126215 | Success: 126178 | No-Balance:     36 | Failed:      0 | RPS:  1303.9 | Success Rate: 100.0% | Latency (ms): avg=1 min=1 max=127
📊 Total: 128753 | Success: 128716 | No-Balance:     36 | Failed:      0 | RPS:  1269.1 | Success Rate: 100.0% | Latency (ms): avg=1 min=0 max=127
📊 Total: 131249 | Success: 131210 | No-Balance:     38 | Failed:      0 | RPS:  1248.1 | Success Rate: 100.0% | Latency (ms): avg=1 min=2 max=127
📊 Total: 133751 | Success: 133707 | No-Balance:     40 | Failed:      0 | RPS:  1251.0 | Success Rate: 100.0% | Latency (ms): avg=1 min=4 max=127
📊 Total: 136344 | Success: 136299 | No-Balance:     44 | Failed:      0 | RPS:  1296.0 | Success Rate: 100.0% | Latency (ms): avg=1 min=0 max=127
📊 Total: 139006 | Success: 138951 | No-Balance:     51 | Failed:      0 | RPS:  1331.6 | Success Rate: 100.0% | Latency (ms): avg=1 min=2 max=127
📊 Total: 141504 | Success: 141450 | No-Balance:     51 | Failed:      0 | RPS:  1248.3 | Success Rate: 100.0% | Latency (ms): avg=1 min=2 max=127
📊 Total: 144077 | Success: 144025 | No-Balance:     52 | Failed:      0 | RPS:  1286.8 | Success Rate: 100.0% | Latency (ms): avg=1 min=1 max=127
📊 Total: 146553 | Success: 146496 | No-Balance:     55 | Failed:      0 | RPS:  1238.1 | Success Rate: 100.0% | Latency (ms): avg=1 min=1 max=127
📊 Total: 149060 | Success: 148996 | No-Balance:     59 | Failed:      0 | RPS:  1253.2 | Success Rate: 100.0% | Latency (ms): avg=1 min=1 max=127
📊 Total: 151624 | Success: 151562 | No-Balance:     62 | Failed:      0 | RPS:  1282.5 | Success Rate: 100.0% | Latency (ms): avg=1 min=0 max=127
📊 Total: 154150 | Success: 154086 | No-Balance:     64 | Failed:      0 | RPS:  1262.4 | Success Rate: 100.0% | Latency (ms): avg=1 min=0 max=127
📊 Total: 156702 | Success: 156635 | No-Balance:     67 | Failed:      0 | RPS:  1276.4 | Success Rate: 100.0% | Latency (ms): avg=1 min=1 max=127
📊 Total: 159243 | Success: 159168 | No-Balance:     72 | Failed:      0 | RPS:  1270.8 | Success Rate: 100.0% | Latency (ms): avg=1 min=2 max=127
📊 Total: 161762 | Success: 161682 | No-Balance:     76 | Failed:      0 | RPS:  1259.4 | Success Rate: 100.0% | Latency (ms): avg=1 min=1 max=127
📊 Total: 164325 | Success: 164245 | No-Balance:     76 | Failed:      0 | RPS:  1281.4 | Success Rate: 100.0% | Latency (ms): avg=1 min=1 max=127
📊 Total: 166679 | Success: 166601 | No-Balance:     77 | Failed:      0 | RPS:  1177.0 | Success Rate: 100.0% | Latency (ms): avg=1 min=0 max=127
📊 Total: 169136 | Success: 169054 | No-Balance:     81 | Failed:      0 | RPS:  1228.5 | Success Rate: 100.0% | Latency (ms): avg=1 min=0 max=127
📊 Total: 171668 | Success: 171583 | No-Balance:     85 | Failed:      0 | RPS:  1266.0 | Success Rate: 100.0% | Latency (ms): avg=1 min=0 max=127
📊 Total: 174124 | Success: 174037 | No-Balance:     86 | Failed:      0 | RPS:  1227.9 | Success Rate: 100.0% | Latency (ms): avg=1 min=0 max=127
📊 Total: 176699 | Success: 176613 | No-Balance:     86 | Failed:      0 | RPS:  1287.7 | Success Rate: 100.0% | Latency (ms): avg=1 min=0 max=127
📊 Total: 179015 | Success: 178915 | No-Balance:     90 | Failed:      0 | RPS:  1158.1 | Success Rate: 100.0% | Latency (ms): avg=1 min=2 max=127
📊 Total: 181432 | Success: 181338 | No-Balance:     93 | Failed:      0 | RPS:  1208.3 | Success Rate: 100.0% | Latency (ms): avg=1 min=1 max=127
📊 Total: 183998 | Success: 183902 | No-Balance:     94 | Failed:      0 | RPS:  1282.8 | Success Rate: 100.0% | Latency (ms): avg=1 min=6 max=127
📊 Total: 186501 | Success: 186399 | No-Balance:     96 | Failed:      0 | RPS:  1251.6 | Success Rate: 100.0% | Latency (ms): avg=1 min=0 max=127
📊 Total: 188936 | Success: 188831 | No-Balance:    100 | Failed:      0 | RPS:  1217.8 | Success Rate: 100.0% | Latency (ms): avg=1 min=1 max=127
📊 Total: 191451 | Success: 191351 | No-Balance:    100 | Failed:      0 | RPS:  1257.1 | Success Rate: 100.0% | Latency (ms): avg=1 min=0 max=127
📊 Total: 193905 | Success: 193803 | No-Balance:    101 | Failed:      0 | RPS:  1227.1 | Success Rate: 100.0% | Latency (ms): avg=1 min=0 max=127
📊 Total: 196318 | Success: 196214 | No-Balance:    102 | Failed:      0 | RPS:  1206.7 | Success Rate: 100.0% | Latency (ms): avg=1 min=0 max=127
📊 Total: 198854 | Success: 198750 | No-Balance:    102 | Failed:      0 | RPS:  1268.1 | Success Rate: 100.0% | Latency (ms): avg=1 min=0 max=127
📊 Total: 201449 | Success: 201343 | No-Balance:    106 | Failed:      0 | RPS:  1297.3 | Success Rate: 100.0% | Latency (ms): avg=1 min=0 max=127
📊 Total: 203938 | Success: 203810 | No-Balance:    115 | Failed:      0 | RPS:  1244.7 | Success Rate: 100.0% | Latency (ms): avg=1 min=1 max=127
📊 Total: 206405 | Success: 206280 | No-Balance:    122 | Failed:      0 | RPS:  1233.5 | Success Rate: 100.0% | Latency (ms): avg=1 min=1 max=127
📊 Total: 209019 | Success: 208889 | No-Balance:    129 | Failed:      0 | RPS:  1307.0 | Success Rate: 100.0% | Latency (ms): avg=1 min=0 max=127
📊 Total: 211565 | Success: 211427 | No-Balance:    133 | Failed:      0 | RPS:  1273.0 | Success Rate: 100.0% | Latency (ms): avg=1 min=1 max=127
📊 Total: 214130 | Success: 213990 | No-Balance:    139 | Failed:      0 | RPS:  1282.1 | Success Rate: 100.0% | Latency (ms): avg=1 min=0 max=127
📊 Total: 216657 | Success: 216513 | No-Balance:    144 | Failed:      0 | RPS:  1263.2 | Success Rate: 100.0% | Latency (ms): avg=1 min=0 max=127
📊 Total: 218946 | Success: 218787 | No-Balance:    151 | Failed:      0 | RPS:  1144.7 | Success Rate: 100.0% | Latency (ms): avg=1 min=2 max=127
📊 Total: 221356 | Success: 221195 | No-Balance:    156 | Failed:      0 | RPS:  1205.1 | Success Rate: 100.0% | Latency (ms): avg=1 min=1 max=127
📊 Total: 223785 | Success: 223621 | No-Balance:    162 | Failed:      0 | RPS:  1214.7 | Success Rate: 100.0% | Latency (ms): avg=1 min=1 max=127
📊 Total: 226328 | Success: 226162 | No-Balance:    165 | Failed:      0 | RPS:  1271.7 | Success Rate: 100.0% | Latency (ms): avg=1 min=1 max=127
📊 Total: 228854 | Success: 228685 | No-Balance:    167 | Failed:      0 | RPS:  1262.5 | Success Rate: 100.0% | Latency (ms): avg=1 min=6 max=127
📊 Total: 231493 | Success: 231320 | No-Balance:    173 | Failed:      0 | RPS:  1319.8 | Success Rate: 100.0% | Latency (ms): avg=1 min=0 max=127
📊 Total: 234084 | Success: 233902 | No-Balance:    178 | Failed:      0 | RPS:  1295.3 | Success Rate: 100.0% | Latency (ms): avg=1 min=3 max=127
📊 Total: 236489 | Success: 236299 | No-Balance:    188 | Failed:      0 | RPS:  1201.4 | Success Rate: 100.0% | Latency (ms): avg=1 min=1 max=127
📊 Total: 239113 | Success: 238911 | No-Balance:    201 | Failed:      0 | RPS:  1313.5 | Success Rate: 100.0% | Latency (ms): avg=1 min=0 max=127
📊 Total: 241718 | Success: 241509 | No-Balance:    204 | Failed:      0 | RPS:  1302.6 | Success Rate: 100.0% | Latency (ms): avg=1 min=1 max=127
📊 Total: 244219 | Success: 244011 | No-Balance:    205 | Failed:      0 | RPS:  1250.5 | Success Rate: 100.0% | Latency (ms): avg=1 min=1 max=127
📊 Total: 246660 | Success: 246446 | No-Balance:    212 | Failed:      0 | RPS:  1220.5 | Success Rate: 100.0% | Latency (ms): avg=1 min=1 max=127
📊 Total: 249091 | Success: 248873 | No-Balance:    215 | Failed:      0 | RPS:  1215.6 | Success Rate: 100.0% | Latency (ms): avg=1 min=2 max=127
📊 Total: 251641 | Success: 251417 | No-Balance:    222 | Failed:      0 | RPS:  1275.0 | Success Rate: 100.0% | Latency (ms): avg=1 min=0 max=127
📊 Total: 254133 | Success: 253906 | No-Balance:    224 | Failed:      0 | RPS:  1245.7 | Success Rate: 100.0% | Latency (ms): avg=1 min=7 max=127
📊 Total: 256630 | Success: 256397 | No-Balance:    231 | Failed:      0 | RPS:  1248.7 | Success Rate: 100.0% | Latency (ms): avg=1 min=0 max=127
📊 Total: 259078 | Success: 258839 | No-Balance:    238 | Failed:      0 | RPS:  1223.9 | Success Rate: 100.0% | Latency (ms): avg=1 min=3 max=127
📊 Total: 261684 | Success: 261437 | No-Balance:    244 | Failed:      0 | RPS:  1302.8 | Success Rate: 100.0% | Latency (ms): avg=1 min=3 max=127
📊 Total: 264101 | Success: 263848 | No-Balance:    248 | Failed:      0 | RPS:  1208.8 | Success Rate: 100.0% | Latency (ms): avg=1 min=2 max=127
📊 Total: 266559 | Success: 266304 | No-Balance:    253 | Failed:      0 | RPS:  1228.3 | Success Rate: 100.0% | Latency (ms): avg=1 min=1 max=127
📊 Total: 268965 | Success: 268696 | No-Balance:    260 | Failed:      0 | RPS:  1203.7 | Success Rate: 100.0% | Latency (ms): avg=1 min=0 max=127
📊 Total: 271528 | Success: 271257 | No-Balance:    268 | Failed:      0 | RPS:  1281.5 | Success Rate: 100.0% | Latency (ms): avg=1 min=1 max=127
📊 Total: 274052 | Success: 273778 | No-Balance:    272 | Failed:      0 | RPS:  1261.9 | Success Rate: 100.0% | Latency (ms): avg=1 min=2 max=127
📊 Total: 276699 | Success: 276424 | No-Balance:    274 | Failed:      0 | RPS:  1323.5 | Success Rate: 100.0% | Latency (ms): avg=1 min=0 max=127
📊 Total: 279259 | Success: 278978 | No-Balance:    280 | Failed:      0 | RPS:  1280.1 | Success Rate: 100.0% | Latency (ms): avg=1 min=1 max=127
📊 Total: 281644 | Success: 281357 | No-Balance:    284 | Failed:      0 | RPS:  1192.4 | Success Rate: 100.0% | Latency (ms): avg=1 min=0 max=127
📊 Total: 284082 | Success: 283787 | No-Balance:    287 | Failed:      0 | RPS:  1218.8 | Success Rate: 100.0% | Latency (ms): avg=1 min=1 max=155
📊 Total: 286644 | Success: 286349 | No-Balance:    293 | Failed:      0 | RPS:  1281.3 | Success Rate: 100.0% | Latency (ms): avg=1 min=2 max=155


````
