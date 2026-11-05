# Awards

Maloja-style awards shown in the header of artist, album and track pages.

- **Yearly medals**: gold, silver and bronze for ranks 1, 2 and 3 of a calendar year. Ties share the rank.
- **Weeks at #1**: number of ISO weeks the item was ranked first.
- **Certification**: gold, platinum or diamond by all-time listen count. An artist gets the higher of its best track's and best album's certification.

The current (open) year and week never count. Each chip links to the chart page of that year.

## Configuration

| Variable | Default | Meaning |
| --- | --- | --- |
| `KOITO_AWARDS_TZ` | `KOITO_FORCE_TZ`, else `UTC` | Timezone used to bucket listens into years and weeks |
| `KOITO_CERT_GOLD` / `_PLATINUM` / `_DIAMOND` | `250` / `500` / `1000` | Track thresholds |
| `KOITO_CERT_GOLD_ALBUM` / `_PLATINUM_ALBUM` / `_DIAMOND_ALBUM` | `500` / `750` / `1500` | Album thresholds |

Thresholds must satisfy gold <= platinum <= diamond, otherwise the feature stays disabled and the error is logged.

## How it works

- Weeks are ISO weeks (Monday start), labelled `YYYY-Www` with the ISO year.
- Closed periods are immutable, so they are computed once into the `awards` table, using upstream's own `GetTop*Paginated` queries.
- A background job runs at startup and every 30 seconds (an idle run is a few small reads). On the first start it backfills every closed year and week since the first listen, in the background; large histories take a while.
- SQLite triggers on `listens`, `artist_tracks` and `tracks.release_id` mark changed periods dirty, so late listens, merges and deletions are picked up by the next run without touching upstream code.
- The tables and triggers are created at startup (`CREATE ... IF NOT EXISTS`), not through a goose migration, to avoid colliding with upstream's migration numbers.

## API

`GET /apis/web/v1/awards/{artist|album|track}/{id}` returns

```json
{"gold": [2024], "silver": [], "bronze": [2022], "weeks_at_one": 3, "certification": "gold"}
```

An unknown id returns the empty summary with HTTP 200.
