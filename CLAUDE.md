# gworkspace — catatan untuk Claude

Domain client tunggal untuk Google Workspace (Calendar + Gmail + Contacts), satu
OAuth refresh token per user. Library generik `go.naturallyfunny.dev/gworkspace`,
dipakai sebagai tool AI agent (traffic rendah). Blueprint: `go.naturallyfunny.dev/spotify`
(pola `TokenStore` + OAuth per-user) dan `go.naturallyfunny.dev/tuya` (domain client
owner-scoped, consumer-defined interface).

## Prinsip yang mengikat (non-negotiable)

1. **Nol coupling ke aplikasi konsumen.** Tidak ada referensi ke aplikasi
   atau identitas app apa pun. User = `owner string` opaque.
2. **`TokenStore` consumer-defined interface** di `client.go`. Library tidak bangun
   credential atau pool DB sendiri — semua via DI di `New`. Implementasi Postgres
   di subpackage `postgres`.
3. **Satu refresh token per user untuk seluruh Workspace** (scope gabungan
   `RequiredScopes`). Bukan satu token per API.
4. **OAuth refresh per-request** (`TokenSourceFor` → `calendarFor`/`gmailFor`/`peopleFor` di subpackage).
5. **`ErrNotConnected`** dari `TokenStore.GetRefreshToken`, di-surface oleh `Client`.
6. Method mengembalikan **tipe domain package ini** (`Event`, `Message`, `Label`,
   `Contact`) — bukan tipe mentah `google.golang.org/api/...`.

## Struktur

```
client.go       Client, Connector, TokenStore, ErrNotConnected/ErrRateLimited,
                RequiredScopes, New, AuthURL/Exchange/Connect, TokenSourceFor, WrapError
calendar/       Service, Event/EventQuery/EventInput, GetEvents, AddEvent
gmail/          Service, Message/Label, ReadMessages, SendEmail, GetLabels,
                ApplyLabel, CreateLabel, GetMessagesByLabel
contact/        Service, Contact/ContactInput, GetContacts, AddContact
postgres/       Store (TokenStore di atas Querier), NewStore/WithAutoMigrate, migrations/
*_test.go       mapping + ErrNotConnected (tanpa network)
```

## OAuth

Konsumen membangun `*oauth2.Config` (Google endpoint + `RequiredScopes`) dan
mengirimnya ke `New(tokenStore, cfg)`. `AuthURL` me-request `AccessTypeOffline`
+ `ApprovalForce` agar Google mengembalikan refresh token. Tiap call fitur bikin
`*Service` Google baru dari refresh token (per-request, lalu dibuang).

## Konteks proyek (kenapa repo ini ada)

Library generik untuk konsumen yang butuh akses Google Workspace (Calendar, Gmail,
Contacts) per-user dengan satu OAuth token. Dirancang sebagai fondasi ADK toolset
(`adk/gcal`, `adk/gmail`, `adk/contact`). Repo ini greenfield.

## Design Decisions

Lihat README "Design Decisions": refresh per-request, limit consumer-controlled via
query structs (zero = no cap, API default applies), refresh token plaintext
(tanggung jawab konsumen), sentinel minimal. Sengaja — bukan bug.

## Verifikasi

`go build ./...`, `go vet ./...`, `go test ./...` harus bersih. Test = mapping murni
+ jalur `ErrNotConnected` (tanpa network). Pakai conventional commits.

## Prev session

Perubahan dari sesi terakhir (belum di-commit):

1. **Rename `ownerID` → `owner` di seluruh permukaan.** Identitas user sengaja
   dinamai `owner` (bukan `ownerID`/`userID`/`subject`): paling general untuk
   konsumen (boleh end-user/tenant/service account, nilai opaque) sekaligus jelas
   — kebetulan ini istilah resmi OAuth2 "resource owner". Suffix `ID` dibuang
   karena menyiratkan indireksi/lookup yang tidak ada. Diterapkan ke semua param
   method, `TokenStore` interface, `postgres` (param Go + kolom DB), migration
   (`owner text PRIMARY KEY`), dan docs. **Konsistensi dengan tuya/spotify sengaja
   diabaikan** — keputusan dinilai dari merit, bukan biar seragam.
2. **`postgres.NewTokenStore`: `validateSchema` hanya jalan saat autoMigrate OFF.**
   Kalau auto-migrate sukses, skema pasti ada — validasi setelahnya redundan.
   Validasi berguna khusus untuk konsumen yang kelola skema sendiri (fail-fast).
3. **`validateSchema` cek kolom eksplisit** (`SELECT owner, refresh_token ...
   LIMIT 0`), bukan cuma keberadaan tabel — tabel-ada-tapi-kolom-salah ikut gagal
   saat startup.

Catatan arah: langkah berikutnya membangun ADK toolset (`gcal`, `gmail`, `contact`)
mengikuti pola `tuya/toolset.go` — interface narrow sisi konsumen, `forAgent(err)`
map sentinel, output `...View`.
