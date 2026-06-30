# gworkspace — catatan untuk Claude

Domain client tunggal untuk Google Workspace (Calendar + Gmail + Contacts), satu
OAuth refresh token per user. Library generik `go.naturallyfunny.dev/gworkspace`,
dipakai sebagai tool AI agent (traffic rendah). Blueprint: `go.naturallyfunny.dev/spotify`
(pola `TokenStore` + OAuth per-user) dan `go.naturallyfunny.dev/tuya` (domain client
owner-scoped, consumer-defined interface).

## Prinsip yang mengikat (non-negotiable)

1. **Nol coupling ke aplikasi konsumen.** Tidak ada referensi ke aplikasi
   atau identitas app apa pun. User = `owner string` opaque.
2. **`TokenStore` consumer-defined interface** di `auth/client.go`. Library tidak bangun
   credential atau pool DB sendiri — semua via DI di `New`. Implementasi Postgres
   di subpackage `auth/postgres`.
3. **Satu refresh token per user untuk seluruh Workspace** (scope gabungan
   `RequiredScopes`). Bukan satu token per API.
4. **OAuth refresh per-request** (`TokenSource` → `calendarFor`/`gmailFor`/`peopleFor`).
5. **`ErrNotConnected`** dari `TokenStore.GetRefreshToken`, di-surface oleh `auth.Client`.
   Didefinisikan di root package (`auth.go`), bukan di `auth/`.
6. Method mengembalikan **tipe domain package ini** (`Event`, `Message`, `Label`,
   `Contact`) — bukan tipe mentah `google.golang.org/api/...`.
7. **Error handling: tiap method urus sendiri** via `fmt.Errorf("op: %w", err)`.
   Tidak ada helper wrapping terpusat. Consumer yang perlu cek kode HTTP pakai
   `errors.As(err, &googleapi.Error{})` langsung — library tidak sembunyikan itu.

## Struktur

```
auth.go         Auth interface, ErrNotConnected, checkScopes
auth/           client.go: Client, TokenStore, ErrMissingScopes, New,
                  AuthURL/Exchange/Connect, TokenSource
                postgres/: Store (TokenStore impl), NewStore/WithAutoMigrate, migrations/
calendar.go     Calendar, Event/EventQuery/EventInput, NewCalendar, GetEvents, AddEvent
gmail.go        Gmail, Message/Label, NewGmail, ReadMessages, SendEmail, GetLabels,
                  ApplyLabel, CreateLabel, GetMessagesByLabel
contact.go      Contacts, Contact/ContactInput, NewContacts, GetContacts, AddContact
*_test.go       mapping + ErrNotConnected (tanpa network)
```

## OAuth

Konsumen membangun `*oauth2.Config` (Google endpoint + `RequiredScopes`) dan
mengirimnya ke `auth.New(store, cfg)`. `AuthURL` me-request `AccessTypeOffline`
+ `prompt=consent` agar Google mengembalikan refresh token. Tiap call fitur bikin
`*Service` Google baru dari refresh token (per-request, lalu dibuang).

## Konteks proyek (kenapa repo ini ada)

Library generik untuk konsumen yang butuh akses Google Workspace (Calendar, Gmail,
Contacts) per-user dengan satu OAuth token. Dirancang sebagai fondasi ADK toolset
(`adk/gcal`, `adk/gmail`, `adk/contact`). Repo ini greenfield.

## Design Decisions

Lihat README "Design Decisions": refresh per-request, limit consumer-controlled via
query structs (zero = no cap, API default applies), refresh token plaintext
(tanggung jawab konsumen). Sengaja — bukan bug.

**Error philosophy:** library tidak wrap atau sembunyikan Google API errors dengan
sentinel buatan sendiri (dulu ada `WrapError` + `ErrRateLimited` — sudah dihapus).
Terlalu abstraktif dan tidak idiomatic Go untuk public library. Consumer punya akses
penuh ke `*googleapi.Error` untuk inspect kode HTTP, message, dll.

## Verifikasi

`go build ./...`, `go vet ./...`, `go test ./...` harus bersih. Test = mapping murni
+ jalur `ErrNotConnected` (tanpa network). Pakai conventional commits.

## Prev session

Perubahan yang sudah ada (belum di-commit):

1. **Rename `ownerID` → `owner` di seluruh permukaan.** Identitas user sengaja
   dinamai `owner` (bukan `ownerID`/`userID`/`subject`): paling general untuk
   konsumen (boleh end-user/tenant/service account, nilai opaque) sekaligus jelas
   — kebetulan ini istilah resmi OAuth2 "resource owner". Suffix `ID` dibuang
   karena menyiratkan indireksi/lookup yang tidak ada. Diterapkan ke semua param
   method, `TokenStore` interface, `postgres` (param Go + kolom DB), migration
   (`owner text PRIMARY KEY`), dan docs.
2. **`postgres.NewTokenStore`: `validateSchema` hanya jalan saat autoMigrate OFF.**
   Kalau auto-migrate sukses, skema pasti ada — validasi setelahnya redundan.
3. **`validateSchema` cek kolom eksplisit** (`SELECT owner, refresh_token ... LIMIT 0`),
   bukan cuma keberadaan tabel.
4. **Hapus `WrapError` dan `ErrRateLimited`.** Tidak idiomatic untuk public library —
   error wrapping urusan tiap method sendiri, consumer handle `*googleapi.Error` langsung.
5. **`ErrNotConnected` dipindah ke root package** (`auth.go`). Sebelumnya salah tempat
   di `auth/client.go`; root package test files dan `auth/postgres` butuh akses ke sini.

Catatan arah: langkah berikutnya membangun ADK toolset (`gcal`, `gmail`, `contact`)
mengikuti pola `tuya/toolset.go` — interface narrow sisi konsumen, `forAgent(err)`
map sentinel, output `...View`.
