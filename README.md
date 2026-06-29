# gworkspace

Module Go `go.naturallyfunny.dev/gworkspace` — reusable public library untuk akses
Google Workspace (Calendar + Gmail + Contacts) atas nama seorang user. Dirancang
sebagai interface-based library agar dapat dipakai lintas project, tidak terikat ke
satu database atau satu aplikasi.

Satu OAuth connect per user mencakup seluruh Workspace: Calendar, Gmail, dan
Contacts berbagi satu refresh token yang di-grant terhadap gabungan
`RequiredScopes`.

## Tujuan Pemakaian (baca sebelum audit)

Library ini dipakai sebagai **tool yang dipanggil oleh AI agent**, bukan backend
high-throughput. Pola traffic-nya: panggilan sporadik, satu aksi per intent user
(baca event hari ini, kirim email, cari contact), volume rendah. Konteks ini
menentukan trade-off di bawah — **jangan menilai repo ini dengan standar service
high-throughput.** Lihat "Design Decisions".

## Struktur

```
client.go     Client, TokenStore interface, ErrNotConnected, RequiredScopes,
              OAuth (AuthURL/Exchange/Connect), xxxFor helper, error mapping
              lintas-fitur (ErrRateLimited, wrapError, sentinelFor)
calendar.go   GetEvents, AddEvent; tipe Event/EventQuery/EventInput
gmail.go      ReadMessages, SendEmail, GetLabels, ApplyLabel, CreateLabel,
              GetMessagesByLabel; tipe Message/Label
contact.go    GetContacts, AddContact; tipe Contact/ContactInput
postgres/
  store.go    implementasi TokenStore di atas Querier (pgxpool), NewStore/WithAutoMigrate
  migrations/ SQL files, di-embed via //go:embed
```

## Cara Pakai

```go
cfg := &oauth2.Config{
    ClientID:     clientID,
    ClientSecret: clientSecret,
    RedirectURL:  redirectURL,
    Endpoint:     google.Endpoint,
    Scopes:       gworkspace.RequiredScopes,
}
store, err := postgres.NewStore(ctx, pool, postgres.WithAutoMigrate())
client := gworkspace.New(store, cfg)

// Connect flow (sekali per user):
url := client.AuthURL(state)        // arahkan user ke sini
err := client.Connect(ctx, owner, code) // di callback

// Pemakaian:
events, err := client.GetEvents(ctx, owner, gworkspace.EventQuery{Query: "standup"})
err = client.SendEmail(ctx, owner, "bob@example.com", "Hi", "body")
contacts, err := client.GetContacts(ctx, owner)
```

User belum connect → method mengembalikan `gworkspace.ErrNotConnected`
(`errors.Is`), supaya konsumen bisa merutekan ke flow login.

## Migrations

Dijalankan **lewat koneksi yang sama** yang di-inject ke `NewStore` (tanpa DSN,
tanpa `golang-migrate`): tiap file `migrations/*.sql` di-`Exec` urut nama. Karena
itu setiap statement **wajib idempoten** (`IF NOT EXISTS` / `IF EXISTS`) — file
di-apply ulang setiap start. Naming: `000N_deskripsi.up.sql`. Tidak ada `.down.sql`
(file `.sql` apa pun di folder ini akan dieksekusi). Jangan edit migration yang
sudah di-commit; tambahkan file baru.

Panggil via `WithAutoMigrate()` saat konstruksi, atau eksplisit dengan
`store.Migrate(ctx)`.

## Design Decisions (sengaja — bukan temuan audit)

Trade-off berikut ditimbang sadar untuk use-case "tool AI agent, traffic rendah".

- **Token refresh per-request** (`xxxFor`, client.go). Tiap panggilan bikin
  `*Service` Google baru dari refresh token lalu dibuang. Aman untuk traffic
  rendah. Caching `*Service`/access-token per-user ditunda sampai ada kebutuhan
  throughput nyata.
- **Limit hardcoded, tanpa paginasi** (events 10, messages 10, contacts 50).
  Cukup untuk satu aksi per intent agent. Paginasi konfigurabel ditunda.
- **Refresh token plaintext** (`postgres/store.go`). Enkripsi-at-rest adalah
  tanggung jawab **konsumen** lewat implementasi `TokenStore`-nya sendiri, bukan
  library.
- **Sentinel minimal.** Hanya `ErrNotConnected` dan `ErrRateLimited` (HTTP 429).
  Sentinel lain (not found, dst) ditambah saat ada konsumen yang perlu branch.

## Conventions

- `TokenStore` interface didefinisikan di `client.go` — consumer-defined interface.
- `postgres.NewStore(ctx, db, opts...)` — ambil `Querier` sempit (pgxpool
  memenuhinya), migrasi lewat koneksi itu sendiri. Tanpa DSN kedua.
- Tidak ada `pkg/` — flat structure.
- Nol referensi ke aplikasi konsumen apa pun; user diidentifikasi lewat
  `owner string` opaque.
