# Upgrading from upstream

The awards feature touches only three upstream files, one hook each:

- `engine/routes.go`: the `/awards/{type}/{id}` route
- `engine/engine.go`: `go awards.Start(ctx, store)`
- `client/app/routes/MediaItems/MediaLayout.tsx`: the `<Awards ... />` element

Everything else is new files (`internal/awards/`, `internal/db/sqlite/awards_access.go`, `client/app/components/Awards.tsx`).

## Procedure

```sh
git remote add upstream https://github.com/gabehf/koito.git   # once
git fetch upstream
git rebase upstream/main
```

Resolve conflicts only at the three hook lines (and their imports), then:

```sh
scripts/check-awards-hooks.sh
go test ./...
docker build .
```

Redeploy afterwards.

## If `go test ./internal/awards/` fails after an upgrade

The first suspects are changed signatures or behaviour of `GetTop*Paginated`, `GetItemsOpts`, `SaveListen`, `SaveArtist`, `SaveAlbum` and `SaveTrack`, and the `*_with_name` / `*_with_title` views the tests use to look items up.

## Upstream migrations that rebuild tables

If a new upstream migration rebuilds `listens`, `tracks` or `artist_tracks` (`DROP` + `RENAME`, as `002_unique_mbid.sql` does for `tracks`), our triggers are lost on that table until the next start, when `EnsureSchema` recreates them. Changes made in between are not recorded, so force a rebuild after deploying such a version (see below).

## Forcing a rebuild

Closed periods are stored once. If upstream changes how rankings count, the stored awards keep the old results until they are rebuilt. To force it, mark everything dirty; the next sync (at most 30 seconds later) recomputes all periods:

```sql
INSERT INTO awards_dirty (ts) VALUES (-1);
```

Changing `KOITO_AWARDS_TZ` triggers this rebuild automatically.
