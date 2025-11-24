# HAProxy Rate Limiting for Nominatim

This setup provides intelligent rate limiting and delay for Nominatim API calls to comply with Nominatim's usage policy while optimizing for ClickHouse performance.

## How It Works

HAProxy adds a **500ms delay** to all Nominatim requests. This allows ClickHouse (which is faster) to respond first and win the race condition when it has a perfect match. This means:

- **SI addresses with ClickHouse matches**: ClickHouse responds quickly, system uses that result, Nominatim request is ignored
- **Addresses without ClickHouse matches**: Nominatim is still queried (with delay), but ClickHouse gets priority
- **Rate limiting**: Only hourly (100) and daily (500) limits are enforced, not per-second (to allow bulk operations)

## Usage

### Normal Mode (Direct Nominatim Access)

Run docker-compose normally without any profile:

```bash
docker-compose up
```

This will use Nominatim directly via the `NOMINATIM_ADDRESS` environment variable (default: `https://nominatim.openstreetmap.org/search`).

### Proxy Mode (With Delay and Rate Limiting)

To use HAProxy with intelligent delay, use the `nominatim-proxy` profile:

```bash
docker-compose --profile nominatim-proxy up
```

This will:
1. Start HAProxy on port 8081
2. Add a 2 second delay to Nominatim requests (allowing ClickHouse to win the race)
3. Enforce rate limits: 100/hour, 500/day (per-second limit removed for bulk operations)

**Important**: When using the proxy profile, you need to set the `NOMINATIM_ADDRESS` environment variable to point to HAProxy:

```bash
NOMINATIM_ADDRESS=http://haproxy:8081/search docker-compose --profile nominatim-proxy up
```

Or set it in your `.env` file:

```env
NOMINATIM_ADDRESS=http://haproxy:8081/search
```

## Rate Limits

HAProxy enforces the following rate limits per source IP:

- **100 requests per hour** - Hourly limit (hard limit)
- **500 requests per day** - Daily limit (hard limit)
- **Per-second limit removed** - Allows bulk validation operations

When a rate limit is exceeded, HAProxy returns HTTP 429 (Too Many Requests) with a JSON error message.

## Delay Mechanism

The 2 second delay is implemented using a Lua script (`delay.lua`) that runs before forwarding requests to Nominatim. This delay:

- Gives ClickHouse time to respond first when it has matches (ClickHouse queries are near-instant as they use in-memory lookups)
- Reduces unnecessary Nominatim API calls (when ClickHouse wins)
- Still allows Nominatim to be used as fallback for addresses not in ClickHouse

## Configuration

- **HAProxy config**: `haproxy.cfg`
- **Lua delay script**: `delay.lua`
- Rate limiting uses HAProxy stick tables that track requests per source IP address

## Notes

- Rate limiting is per source IP address
- When running in Docker, the source IP will be the OpenPAQ container's IP
- The delay helps ClickHouse win the race condition, reducing Nominatim load
- Retry logic in the Go code handles 429 errors with exponential backoff
- For production use with multiple clients, consider using a self-hosted Nominatim instance instead
- The proxy forwards requests to `https://nominatim.openstreetmap.org` over HTTPS

