# Auth Rosetta

A multi-stack monorepo exploring how different backend ecosystems natively implement the exact same Two-Factor Authentication (TOTP) login flow. 

This project serves as a "Rosetta Stone" for backend development. It uses a single React SPA that can be hot-swapped to point at different backend APIs. Each backend connects to its own isolated PostgreSQL database, allowing for a pure comparison of how each language and framework handles database migrations, password hashing, and TOTP secret generation.

## Architecture

* **Frontend:** React + Vite (TypeScript)
* **Infrastructure:** Docker Compose (Shared PostgreSQL container with logical DB isolation)
* **Backends:**
  * **.NET 10** (C# + Entity Framework Core) - *In Progress*
  * **NestJS** (Node + TypeScript) - *Planned*
  * **Python** - *Planned*
  * **Go** - *Planned*
  * **Java** (Spring Boot) - *Planned*

## Getting Started

### 1. Database Setup
The project uses a single Postgres container that automatically initializes isolated databases for each stack (`auth_net`, `auth_nest`, `auth_python`, `auth_go`, `auth_java`).

Create a `.env` file in the root directory with your database credentials:
```env
DB_USER=rosetta_admin
DB_PASSWORD=your_secure_password
```

Spin up the database infrastructure:
```bash
docker compose up -d
```

### 2. Frontend Setup
*(Instructions pending frontend implementation)*

### 3. Backend Setup
*(Instructions pending individual backend implementations)*

## Project Structure

```text
auth-rosetta/
├── docker-compose.yml       # Shared database infrastructure
├── init-databases.sql       # Postgres initialization script
├── .env                     # Local secrets (ignored in git)
├── .env.example             # Example .env file for reference
├── frontend-react/          # The shared React application
├── backend-net/             # .NET 10 implementation
├── backend-nest/            # NestJS implementation
├── backend-python/          # Python implementation
├── backend-go/              # Go implementation
└── backend-java/            # Java implementation
```