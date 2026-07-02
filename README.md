OpenPAQ (**Open P**ostal **A**ddress **Q**uality) is a tool designed to validate key components of postal addresses, ensuring data accuracy and integrity. A core capability of OpenPAQ is the normalization of addresses as they are found "in the wild." It transforms diverse and inconsistent address inputs into a standardized format that is valid for performing checks against the OpenStreetMap database.

Operating via a simple HTTP-Endpoint, OpenPAQ leverages [Nominatim](https://github.com/osm-search/Nominatim) in the background to perform its checks. It is tested against a self-hosted Nominatim version from [mediagis](https://github.com/mediagis/nominatim-docker)..

OpenPAQ checks the following address components:


- street
- city
- postal code
- country code

### Key Features of OpenPAQ:

OpenPAQ offers the following capabilities in address validation, measured with an internal benchmark of 90 correct and 40 incorrect postal addresses per country:
1.	International Address Validation: Provides address validation with accuracy levels of approximately 80% or higher for key European countries (DE, NL, AT, CH, FR, GB, IT, PL, DK)
2.	Correct Address Identification: Achieves a recall rate of approximately 75% or higher for most benchmarked countries in identifying correct addresses.
3.	Incorrect Address Detection: Offers an F1 score of over 75% for most benchmarked countries in identifying incorrect addresses.


Please have a look at the [documentation](https://openpaq.de) for a detailed description of the program.

## Docker Compose Setup

This project includes a `docker-compose.yml` file that sets up the OpenPAQ server and ClickHouse database for Slovenian address validation. It also uses GitLab CI/CD to build and deploy, synced automatically from GitHub.

### Prerequisites

- Docker and Docker Compose installed
- Slovenian address data to import into ClickHouse

### Quick Start

1. **Start the services:**
   ```bash
   docker-compose up -d
   ```

2. **Verify ClickHouse is running:**
   ```bash
   docker-compose ps
   ```

3. **Import your Slovenian address data:**
   The table `slovenian_addresses` will be automatically created. You need to import your data using ClickHouse client:
   ```bash
   docker-compose exec clickhouse clickhouse-client
   ```
   Then use `INSERT` statements or import from a file.

### Configuration

The docker-compose setup uses the following default ClickHouse configuration:
- **Host:** `clickhouse` (internal Docker network)
- **Port:** `9000` (native protocol)
- **Database:** `default`
- **Table:** `slovenian_addresses`
- **User:** `default`
- **Password:** `default`

You can override these settings by modifying the environment variables in `docker-compose.yml` or by creating a `.env` file.

### Environment Variables

Key environment variables for OpenPAQ (set in docker-compose.yml):
- `CLICKHOUSE_ENABLED=true` - Enables ClickHouse integration
- `CLICKHOUSE_DB_HOST=clickhouse` - ClickHouse hostname
- `CLICKHOUSE_DB_PORT=9000` - ClickHouse native port
- `CLICKHOUSE_DB_DATABASE=default` - Database name
- `CLICKHOUSE_DB_TABLE=slovenian_addresses` - Table name
- `CLICKHOUSE_COUNTRY=de` - Country-specific matcher to load (`de` default, set to `si` to enable the Slovenian schema)
- `NOMINATIM_ADDRESS=https://nominatim.openstreetmap.org/search` - Default Nominatim endpoint
- `WEBSERVER_LISTEN_ADDRESS` - Server listen address (default: `0.0.0.0:8080`)

### Data Persistence

ClickHouse data is persisted in a Docker volume named `clickhouse_data`. To remove all data:
```bash
docker-compose down -v
```

### Troubleshooting

- **Check ClickHouse logs:**
  ```bash
  docker-compose logs clickhouse
  ```

- **Check OpenPAQ logs:**
  ```bash
  docker-compose logs openpaq
  ```

- **Verify table creation:**
  ```bash
  docker-compose exec clickhouse clickhouse-client --query "SHOW TABLES"
  ```
