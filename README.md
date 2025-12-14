# Memcortex

**Persistent Memory Layer for LLMs (Memory-RAG)**

Memcortex is a Proof of Concept (PoC) designed to equip conversational agents and LLM applications with persistent, long-term memory. By implementing a Memory-RAG (Retrieval-Augmented Generation) architecture, Memcortex allows agents to transcend context-window limitations, enabling them to recall past interactions and specific data points indefinitely.

[![Read the Deep Dive on Medium](https://img.shields.io/badge/Medium-Read_The_Article_Here-blue?style=for-the-badge&logo=medium)](https://medium.com/@sobowalebukola/inside-memcortex-a-lightweight-semantic-memory-layer-for-llms-394cf940191a)

---

## Contents

* README (this file)
* Architectural diagram (Mermaid + ASCII)
* Package structure
* How to start

---

## Project Overview

Memcortex stores user/application memories as both text and vectors in Weaviate and exposes a memory manager + middleware that:

1. Embeds incoming text using `nomic-embed-text` embeddings.
2. Stores memories in a `Memory_idx` class on Weaviate.
3. Runs vector searches to retrieve top‑K relevant memories for a user.
4. Injects retrieved memories into the prompt before it reaches the LLM.
5. Optionally persists new memories asynchronously.

This pattern is ideal for building chatbots, agents, and personalization layers that must "remember" details across sessions.

<img width="972" height="321" alt="memcortex drawio" src="https://github.com/user-attachments/assets/51b25a3a-93cd-4e5d-883b-5af99da2627b" />


---

## Architecture (Mermaid)

```mermaid
flowchart LR
  A[User] -->|POST /chat| B(API Server)
  B --> C{Memory Middleware}
  C -->|retrieve| D[Weaviate Vector Store]
  D -->|top-K| C
  C -->|inject memories| E[LLM Handler]
  E -->|call LLM API| F[Ollama / Custom LLM]
  F -->|response| E
  E -->|save message| G(Background Save Worker)
  G --> D
  subgraph Infra
    D
    F
  end
```

### ASCII Diagram

```
User -> API Server (/chat)
      -> Memory Middleware:
           - Embed user query via Ollama
           - Query Weaviate vector index (top-K)
           - Re-rank / filter / format
           - Inject into prompt
      -> LLM Handler -> Local or remote LLM
      -> Return response
Background worker: saves new user messages into Weaviate (embedding -> object)
```

---

## Quickstart (developer)

Prereqs:

* Go 1.20+
* Docker & Docker Compose

1. Copy repo and set module path (or `go mod init github.com/yourname/memcortex`).
2. Create `.env` (see `.env.example`).
3. Build docker image & run server:

```bash
docker-compose up -d --build
```
<img width="1399" height="165" alt="Screenshot 2025-12-09 at 10 54 59 PM" src="https://github.com/user-attachments/assets/a7629e4e-ed5c-4955-abca-cbce7dbf09e1" />


5. Example request:

```bash
curl -X POST http://localhost:8080/chat \
  -H "Content-Type: application/json" \
  -H "X-User-ID: memcortex-user-x" \
  -d '{"message":"My preffered memory layer is memcortex."}'
```
Shown below are example requests using Thunderclient (you can use any api client of choice. Remember to set the `X-User-ID` in the headers)

<img width="1461" height="727" alt="Screenshot 2025-12-09 at 10 49 32 PM" src="https://github.com/user-attachments/assets/10e21b48-c0fd-4863-bc4c-61dfc9a4f8b0" />
<img width="1461" height="733" alt="Screenshot 2025-12-09 at 10 49 07 PM" src="https://github.com/user-attachments/assets/9703c179-0a66-4099-8965-2cfd6232365f" />
<img width="1463" height="732" alt="Screenshot 2025-12-09 at 10 48 45 PM" src="https://github.com/user-attachments/assets/80529175-fbda-4859-90f4-c6e29f31fc92" />
<img width="1463" height="732" alt="Screenshot 2025-12-09 at 10 48 13 PM" src="https://github.com/user-attachments/assets/3fc418e2-5cee-479b-9b2c-7689b08a3dfd" />


<img width="1454" height="724" alt="Screenshot 2025-12-09 at 10 53 23 PM" src="https://github.com/user-attachments/assets/243a53d8-ad78-4d2a-86da-8bd3ff117657" />
<img width="1463" height="724" alt="Screenshot 2025-12-09 at 10 52 20 PM" src="https://github.com/user-attachments/assets/64a50f36-ac47-45bc-8f40-0cb67d1cb33f" />
<img width="1457" height="729" alt="Screenshot 2025-12-09 at 10 51 48 PM" src="https://github.com/user-attachments/assets/66536fe8-de24-48d6-8317-cd72bd3a4b3b" />
<img width="1460" height="731" alt="Screenshot 2025-12-09 at 10 51 23 PM" src="https://github.com/user-attachments/assets/dda80703-5bc6-4cef-917a-867ad535a6df" />


The first request will save the memory asynchronously. Later requests will retrieve and inject the memory.

---

## Package structure

```
memcortex/
├─ cmd/server/main.go          # App entry point
├─ internal/
│  ├─ embedder/
│  │  └─ ollama.go             # Contains OpenAI Embedder Logic (for text-embedding-3-small)
│  ├─ handlers/
│  │  └─ chat.go               # Chat endpoint handler
│  ├─ memory/
│  │  ├─ manager.go            # High-level RAG orchestration
│  │  ├─ queue.go              
│  │  └─ store.go              # Weaviate storage wrapper
│  └─ middleware/
│     └─ memory_middleware.go  # Context injection middleware
├─ .env.example                      # Environment file
├─ docker-compose.yml
├─ Dockerfile
├─ Dockerfile.ollama
├─ go.mod
├─ go.sum
└─ README.md
```
---

### .env.example

```
EMBEDDING_MODEL=nomic-embed-text
EMBEDDING_DIM=768
SERVER_ADDR=:8080
OLLAMA_ADDR=11434
MAX_MEMORY_DISTANCE=0.5 // This describes the vector search distance 
TOP_K_MEMORIES=10
```

---

### docker-compose.yml

```yaml
services:
  ollama:
    build:
      context: .
      dockerfile: Dockerfile.ollama
    container_name: ollama
    ports:
      - "${OLLAMA_ADDR}:11434"
    restart: unless-stopped
    entrypoint: ["/bin/sh", "-c"]
    command: >
      "ollama serve & 
      sleep 5 && 
      ollama pull ${EMBEDDING_MODEL} && 
      wait"
    volumes:
      - /root/.ollama
    healthcheck:
      test: ["CMD", "ollama", "list"]
      interval: 10s
      timeout: 5s
      retries: 5

  weaviate:
    image: semitechnologies/weaviate:1.25.3
    container_name: weaviate
    ports:
      - "6379:8080"
      - "50051:50051"
    environment:
      QUERY_DEFAULTS_LIMIT: 25
      AUTHENTICATION_ANONYMOUS_ACCESS_ENABLED: "true"
      PERSISTENCE_DATA_PATH: "var/lib/weaviate"
      DEFAULT_VECTORIZER_MODULE: "none"
      CLUSTER_HOSTNAME: "node1"
    volumes:
      - /var/lib/weaviate
    restart: unless-stopped
  go-server:
    build:
      context: ./
      dockerfile: Dockerfile
    container_name: go-server
    ports:
      - "${SERVER_ADDR}:8080"
    environment:
      - OLLAMA_HOST=http://ollama:11434
      - EMBEDDING_MODEL=nomic-embed-text
      - WEAVIATE_HOST=http://weaviate:8080
    depends_on:
      ollama:
        condition: service_healthy
      weaviate:
        condition: service_started
    restart: unless-stopped
```
